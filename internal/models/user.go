package models

import (
	"github.com/velocitykode/velocity/orm"
)

// User is the application's account row and the auth model registered with
// velocity.SetAuthModel.
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
// path the user store prefers: lookups hand the model straight back and the
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

// ProtectedFields opts out of velocity's deny-by-default mass assignment with
// an empty denylist (allow-all; an allowlist would zero unlisted fields on
// struct writes). Name a column here to keep map-based writes from ever
// reaching it.
func (User) ProtectedFields() []string { return nil }
