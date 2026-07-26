package models

import (
	"github.com/velocitykode/velocity/auth/providers/ormauth"
	"github.com/velocitykode/velocity/orm"
)

// The auth provider selects its model through a name registry, because the
// ORM needs a compile-time type where configuration only carries a string
// (AUTH_MODEL). "User" ships pre-registered to the framework's placeholder
// model; binding it here is what makes authenticated requests carry THIS
// model instead of the stand-in.
func init() {
	ormauth.MustRegister(ormauth.DefaultModelName, ormauth.Factory[User]())
}

// User model represents a user in the application
type User struct {
	orm.Model[User]
	Name     string `orm:"column:name;type:varchar(255);not_null" json:"name"`
	Email    string `orm:"column:email;type:varchar(255);unique;not_null" json:"email"`
	Password string `orm:"column:password;type:varchar(255);not_null" json:"-"`
	// Nullable: a user who has never used remember-me holds SQL NULL here,
	// and scanning that into a plain string is a driver error.
	RememberToken *string `orm:"column:remember_token;type:varchar(255)" json:"-"`
}

// Authenticatable implementation. Implementing the interface directly is the
// path the provider prefers: lookups hand the model straight back and the
// reflection-based column mapping is skipped entirely.

// GetAuthIdentifier returns the primary key.
func (u *User) GetAuthIdentifier() interface{} { return u.ID }

// GetAuthPassword returns the stored password hash.
func (u *User) GetAuthPassword() string { return u.Password }

// GetRememberToken returns the remember-me token, or "" when unset.
func (u *User) GetRememberToken() string {
	if u.RememberToken == nil {
		return ""
	}
	return *u.RememberToken
}

// SetRememberToken sets the remember-me token.
func (u *User) SetRememberToken(token string) {
	u.RememberToken = &token
}

// TableName specifies the table name for the User model
func (User) TableName() string {
	return "users"
}

// Guarded opts out of velocity deny-by-default mass assignment with an empty
// denylist (allow-all, no Fillable acronym-zeroing). Name a column here to keep
// map-based writes from ever reaching it.
func (User) Guarded() []string { return nil }
