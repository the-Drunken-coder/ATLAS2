//go:build unix

package objectstorage

import "syscall"

const noFollowOpenFlag = syscall.O_NOFOLLOW
