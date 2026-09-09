package guest

import (
	"net/http"
	"strconv"

	v1_catalogpb "github.com/menli02/QR-menu/proto/catalog/v1"
	"github.com/menli02/QR-menu/services/gateway/internal/authz"
	"github.com/menli02/QR-menu/services/gateway/internal/convert"
	"github.com/menli02/QR-menu/services/gateway/internal/rpcerr"
	"github.com/menli02/QR-menu/services/gateway/internal/svc"
	"github.com/menli02/QR-menu/services/gateway/internal/types"

	"github.com/zeromicro/go-zero/rest/httpx"
)

// menuMaxAgeSeconds is §8.1's `Cache-Control: max-age=60` for the guest
// menu. Sixty seconds of staleness is fine for prices and descriptions;
// availability is the volatile part, and that arrives over the WebSocket
// menu channel within seconds (FR-C4) rather than waiting for a re-fetch.
const menuMaxAgeSeconds = 60

// MenuCacheHandler replaces the goctl-generated GetGuestMenu handler so
// the route can set the ETag and Cache-Control headers §8.1 asks for.
//
// It exists as a hand-written handler because a logic function's signature
// is (ctx, req) -> (resp, error): no ResponseWriter, so no way to set a
// header or return 304. Everything except the caching lives in the logic
// layer as usual; this wrapper only adds the parts that need the
// ResponseWriter.
//
// The ETag is catalog's menu_version, which is exactly the right value:
// it is a counter bumped in the same transaction as any write that changes
// what the guest menu shows, so equal versions genuinely mean equal
// menus. Inventing a content hash here would be slower and no more
// correct.
//
// It is a weak validator (W/) on purpose. Two responses with the same
// menu_version are semantically the same menu, but not necessarily
// byte-identical — a different `locale` produces different text. Claiming
// strong equivalence would be a lie, and the one thing a strong ETag
// enables (range requests) is meaningless for a JSON document.
func MenuCacheHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.GetGuestMenuReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		claims, err := authz.Guest(r.Context())
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		// The client's cached version is handed to catalog, which
		// short-circuits and returns not_modified without assembling the
		// menu at all. That is the point of doing this here rather than
		// comparing after the fact: a matching ETag should cost a version
		// lookup, not a full menu build that is then thrown away.
		menu, err := svcCtx.CatalogRpc.GetMenu(r.Context(), &v1_catalogpb.GetMenuRequest{
			VenueId:       claims.VenueID,
			Locale:        req.Locale,
			IfMenuVersion: menuVersionFromETag(r.Header.Get("If-None-Match")),
		})
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, rpcerr.FromCatalog(err))
			return
		}

		setMenuCacheHeaders(w, menu.GetMenuVersion())

		if menu.GetNotModified() {
			// 304 carries no body, and RFC 9110 requires the validators to
			// be present — which setMenuCacheHeaders has already done.
			w.WriteHeader(http.StatusNotModified)
			return
		}

		out := &types.GetGuestMenuResp{
			VenueId:     menu.GetVenueId(),
			MenuVersion: menu.GetMenuVersion(),
			Currency:    menu.GetCurrency(),
			Categories:  make([]types.MenuCategory, 0, len(menu.GetCategories())),
		}
		for _, c := range menu.GetCategories() {
			category := c.GetCategory()
			out.Categories = append(out.Categories, types.MenuCategory{
				Id:        category.GetId(),
				Name:      convert.Localized(category.GetName(), req.Locale, ""),
				SortOrder: category.GetSortOrder(),
				Items:     convert.MenuItems(c.GetItems(), req.Locale, ""),
			})
		}
		httpx.OkJsonCtx(r.Context(), w, out)
	}
}

func setMenuCacheHeaders(w http.ResponseWriter, menuVersion string) {
	w.Header().Set("ETag", etagForMenu(menuVersion))

	// `private`, not `public`: the venue this returns comes from the
	// caller's token, so two guests at different venues get different
	// menus from the same URL. A shared cache that stored one and served
	// it to the other would show a guest the wrong restaurant's menu.
	//
	// Vary: Authorization says the same thing in the language a proxy
	// understands. Locale needs no Vary — it is a query parameter, so it
	// is already part of the cache key.
	w.Header().Set("Cache-Control", "private, max-age="+strconv.Itoa(menuMaxAgeSeconds))
	w.Header().Set("Vary", "Authorization, Accept-Encoding")
}

func etagForMenu(menuVersion string) string {
	if menuVersion == "" {
		return ""
	}
	return `W/"` + menuVersion + `"`
}

// menuVersionFromETag unwraps an If-None-Match value back to the bare
// menu_version catalog understands.
//
// Tolerant of both the wrapped and bare forms: a client that echoes back
// the `menuVersion` from the response body rather than the ETag header is
// doing something reasonable, and there is no reason to make that miss.
// A list of several etags, or `*`, is ignored — neither is meaningful for
// this route, and guessing at one would be worse than a cache miss.
func menuVersionFromETag(header string) string {
	if header == "" || header == "*" {
		return ""
	}
	v := header
	if len(v) > 2 && v[0] == 'W' && v[1] == '/' {
		v = v[2:]
	}
	if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
		v = v[1 : len(v)-1]
	}
	// A comma means the client sent several validators. This route has one
	// meaningful version at a time, so rather than picking arbitrarily,
	// treat it as no validator and serve a full response.
	for i := 0; i < len(v); i++ {
		if v[i] == ',' {
			return ""
		}
	}
	return v
}
