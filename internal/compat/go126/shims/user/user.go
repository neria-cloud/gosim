//go:build go1.26 && !go1.27

// Package user keeps optional account lookup fail-closed.
package user

import (
	"fmt"

	gosimruntime "github.com/neria-cloud/gosim/rt/runtime"
	"github.com/neria-cloud/gosim/rt/runtime/abi"
)

type (
	User               struct{ Uid, Gid, Username, Name, HomeDir string }
	Group              struct{ Gid, Name string }
	UnknownUserIdError int
)

func (e UnknownUserIdError) Error() string { return fmt.Sprintf("user: unknown userid %d", e) }

type UnknownUserError string

func (e UnknownUserError) Error() string { return "user: unknown user " + string(e) }

type UnknownGroupIdError string

func (e UnknownGroupIdError) Error() string { return "group: unknown groupid " + string(e) }

type UnknownGroupError string

func (e UnknownGroupError) Error() string { return "group: unknown group " + string(e) }

func unsupported() {
	gosimruntime.Abort(abi.Diagnostic{Kind: "UnsupportedOperation", Feature: "rt", Operation: "os/user", Message: "host account lookup is not modeled"})
}
func Current() (*User, error)                  { unsupported(); return nil, nil }
func Lookup(username string) (*User, error)    { unsupported(); return nil, nil }
func LookupId(uid string) (*User, error)       { unsupported(); return nil, nil }
func LookupGroup(name string) (*Group, error)  { unsupported(); return nil, nil }
func LookupGroupId(gid string) (*Group, error) { unsupported(); return nil, nil }
func (u *User) GroupIds() ([]string, error)    { unsupported(); return nil, nil }
