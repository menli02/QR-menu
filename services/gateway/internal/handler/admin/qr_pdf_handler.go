// Package admin holds the hand-written admin handlers — the ones whose
// responses are not JSON, so goctl's logic signature cannot produce them.
package admin

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	v1_catalogpb "github.com/menli02/QR-menu/proto/catalog/v1"
	"github.com/menli02/QR-menu/services/gateway/internal/authz"
	"github.com/menli02/QR-menu/services/gateway/internal/errs"
	"github.com/menli02/QR-menu/services/gateway/internal/rpcerr"
	"github.com/menli02/QR-menu/services/gateway/internal/svc"

	"github.com/go-pdf/fpdf"
	"github.com/skip2/go-qrcode"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// Layout: a 2x3 grid of table cards on A4 portrait, which gives a QR big
// enough to scan from across a table and a label readable while holding
// the sheet.
const (
	pageWidthMM   = 210.0
	pageHeightMM  = 297.0
	marginMM      = 12.0
	cardsPerRow   = 2
	cardsPerCol   = 3
	qrSizePx      = 512 // rendered raster size; scaled to the card by fpdf
	qrRecoveryLvl = qrcode.Medium
)

// QRPDFHandler renders a printable sheet of table QR codes (FR-T3,
// GET /admin/tables/qr.pdf).
//
// This route returned 501 until catalog grew GetTablePrintCodes. The
// blocker was not rendering: it was that nothing in the contract exposed a
// table's HMAC signature, and the gateway cannot derive one because the
// signing key lives in catalog and never leaves it.
//
// Handwritten rather than generated because the response is a PDF. Two
// consequences worth stating:
//
//   - The URL encoded into each QR is the *public* guest URL, not this
//     gateway's own address. A guest scans it with a phone that has never
//     heard of the cluster, so it has to be the address the frontend is
//     served from — hence the configured base URL rather than r.Host,
//     which behind an ingress is whatever the proxy passed along.
//   - The sheet carries live credentials. Anyone holding the printout can
//     open a guest session at those tables, which is exactly what it is
//     for, and exactly why the response is marked no-store: a PDF of a
//     venue's QR codes sitting in a shared proxy cache is a problem.
func QRPDFHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, err := authz.Admin(r.Context())
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		baseURL := strings.TrimRight(svcCtx.Config.GuestBaseURL, "/")
		if baseURL == "" {
			// Refusing beats emitting a sheet of QR codes pointing at a
			// URL that does not resolve. Those get printed, laminated and
			// stuck to tables before anyone scans one.
			httpx.ErrorCtx(r.Context(), w, errs.New(errs.CodeInternal,
				"guest base URL is not configured; QR codes cannot be generated"))
			return
		}

		// An explicit ?tableIds=a,b prints a subset — the case where one
		// table's sticker was damaged and nobody wants to reprint the
		// venue. Empty means every active table.
		var tableIDs []string
		if raw := r.URL.Query().Get("tableIds"); raw != "" {
			for _, id := range strings.Split(raw, ",") {
				if id = strings.TrimSpace(id); id != "" {
					tableIDs = append(tableIDs, id)
				}
			}
		}

		codes, err := svcCtx.CatalogRpc.GetTablePrintCodes(r.Context(), &v1_catalogpb.GetTablePrintCodesRequest{
			VenueId:      claims.VenueID,
			TableIds:     tableIDs,
			ActorStaffId: claims.StaffID,
		})
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, rpcerr.FromCatalog(err))
			return
		}
		if len(codes.GetCodes()) == 0 {
			httpx.ErrorCtx(r.Context(), w, errs.New(errs.CodeNotFound,
				"no active tables to print"))
			return
		}

		pdf, err := renderQRSheet(baseURL, codes)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, errs.New(errs.CodeInternal, "could not render the QR sheet"))
			return
		}

		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition",
			fmt.Sprintf(`attachment; filename="qr-codes-%s.pdf"`, sanitizeFilename(codes.GetVenueSlug())))
		// The sheet is a bearer credential for every table on it.
		w.Header().Set("Cache-Control", "no-store, private")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(pdf)
	}
}

// guestURL builds the link a QR encodes: the guest frontend, carrying the
// three values POST /guest/sessions needs to exchange for a token.
//
// key_version travels prefixed onto the signature ("<version>.<sig>")
// rather than as a fourth parameter, matching what the session-exchange
// handler parses. FR-T4 keeps old keys valid through a grace period, so
// verification has to know which key signed a given sticker — and the
// sticker is the only place that information can live.
func guestURL(baseURL, venueSlug string, c *v1_catalogpb.TablePrintCode) string {
	q := url.Values{}
	q.Set("v", venueSlug)
	q.Set("t", c.GetTableCode())
	q.Set("s", fmt.Sprintf("%d.%s", c.GetKeyVersion(), c.GetSig()))
	return baseURL + "/?" + q.Encode()
}

func renderQRSheet(baseURL string, codes *v1_catalogpb.GetTablePrintCodesResponse) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetTitle("QR codes — "+codes.GetVenueSlug(), true)
	// Deterministic output: without a fixed creation date the same input
	// produces a different file every call, which defeats any caching or
	// diffing downstream and makes the endpoint untestable by comparison.
	pdf.SetCreationDate(fixedPDFDate())

	cardW := (pageWidthMM - 2*marginMM) / cardsPerRow
	cardH := (pageHeightMM - 2*marginMM) / cardsPerCol
	perPage := cardsPerRow * cardsPerCol

	for i, c := range codes.GetCodes() {
		if i%perPage == 0 {
			pdf.AddPage()
		}
		slot := i % perPage
		x := marginMM + float64(slot%cardsPerRow)*cardW
		y := marginMM + float64(slot/cardsPerRow)*cardH

		png, err := qrcode.Encode(guestURL(baseURL, codes.GetVenueSlug(), c), qrRecoveryLvl, qrSizePx)
		if err != nil {
			return nil, fmt.Errorf("encode QR for table %s: %w", c.GetTableId(), err)
		}

		// Registered under the table id so fpdf embeds each image once
		// even if the same sheet is regenerated.
		name := "qr-" + c.GetTableId()
		pdf.RegisterImageOptionsReader(name, fpdf.ImageOptions{ImageType: "PNG"}, bytes.NewReader(png))

		qrSide := cardH * 0.62
		pdf.ImageOptions(name, x+(cardW-qrSide)/2, y+8, qrSide, qrSide,
			false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")

		pdf.SetXY(x, y+qrSide+12)
		pdf.SetFont("Helvetica", "B", 20)
		pdf.CellFormat(cardW, 9, c.GetLabel(), "", 2, "C", false, 0, "")

		pdf.SetFont("Helvetica", "", 9)
		pdf.SetTextColor(110, 110, 110)
		pdf.CellFormat(cardW, 5, codes.GetVenueSlug(), "", 2, "C", false, 0, "")
		// The key version is printed in the clear so that after a rotation
		// staff can tell at a glance which stickers are the old ones,
		// without scanning every table.
		pdf.CellFormat(cardW, 5, fmt.Sprintf("key v%d", c.GetKeyVersion()), "", 2, "C", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("write pdf: %w", err)
	}
	return buf.Bytes(), nil
}

// sanitizeFilename keeps a venue slug safe to interpolate into a
// Content-Disposition header. A slug is already constrained, but this
// header is one of the classic response-splitting surfaces and the check
// costs nothing.
func sanitizeFilename(s string) string {
	out := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			return r
		default:
			return '-'
		}
	}, s)
	if out == "" {
		return "venue"
	}
	return out
}

// fixedPDFDate pins the PDF's creation timestamp so the same tables
// always render byte-identically. See renderQRSheet.
func fixedPDFDate() time.Time {
	return time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
}
