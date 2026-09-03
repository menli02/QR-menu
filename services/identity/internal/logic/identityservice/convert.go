package identityservicelogic

import (
	"fmt"

	"github.com/menli02/QR-menu/proto/identity/v1"
	"github.com/menli02/QR-menu/services/identity/internal/model"

	"google.golang.org/protobuf/types/known/timestamppb"
)

// roleToDB maps the wire Role enum to the lowercase string stored in
// identity_db.staff.role (see that table's CHECK constraint).
// ROLE_UNSPECIFIED is rejected: every staff row must have a real role.
func roleToDB(r v1_identitypb.Role) (string, error) {
	switch r {
	case v1_identitypb.Role_ROLE_ADMIN:
		return "admin", nil
	case v1_identitypb.Role_ROLE_MANAGER:
		return "manager", nil
	case v1_identitypb.Role_ROLE_WAITER:
		return "waiter", nil
	case v1_identitypb.Role_ROLE_COOK:
		return "cook", nil
	default:
		return "", fmt.Errorf("unspecified or unknown role %v", r)
	}
}

func roleFromDB(s string) v1_identitypb.Role {
	switch s {
	case "admin":
		return v1_identitypb.Role_ROLE_ADMIN
	case "manager":
		return v1_identitypb.Role_ROLE_MANAGER
	case "waiter":
		return v1_identitypb.Role_ROLE_WAITER
	case "cook":
		return v1_identitypb.Role_ROLE_COOK
	default:
		return v1_identitypb.Role_ROLE_UNSPECIFIED
	}
}

// staffToProto renders a model.Staff row as the wire Staff message. It
// never copies PasswordHash — that field has no proto counterpart.
func staffToProto(s *model.Staff) *v1_identitypb.Staff {
	return &v1_identitypb.Staff{
		Id:        s.ID,
		VenueId:   s.VenueID,
		Name:      s.Name,
		Email:     s.Email,
		Role:      roleFromDB(s.Role),
		IsActive:  s.IsActive,
		CreatedAt: timestamppb.New(s.CreatedAt),
		UpdatedAt: timestamppb.New(s.UpdatedAt),
	}
}
