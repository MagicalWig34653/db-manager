// SPDX-License-Identifier: AGPL-3.0-only
package sqlitesandbox

import (
	"errors"
	"io/fs"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"

	"database/sql"
	sqlite "github.com/ncruces/go-sqlite3"
	"github.com/ncruces/go-sqlite3/driver"
	"github.com/ncruces/go-sqlite3/util/vfsutil"
	"github.com/ncruces/go-sqlite3/vfs"
	"golang.org/x/sys/unix"
)

var validID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)
var nextVFS atomic.Uint64

// Open limits SQL to a single database and rollback journal under root.
// The experimental VFS requires exclusive ownership of the database volume.
func Open(rootPath, id string) (*sql.DB, func(), error) {
	if !validID.MatchString(id) {
		return nil, nil, fs.ErrPermission
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return nil, nil, err
	}
	name := "dbmanager-sandbox-" + strconv.FormatUint(nextVFS.Add(1), 10)
	restricted := &rootVFS{root: root, database: id + ".sqlite"}
	vfs.Register(name, restricted)
	cleanup := func() { vfs.Unregister(name); root.Close() }
	db, err := driver.Open("file:"+id+".sqlite?vfs="+name, configure)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	return db, cleanup, nil
}

func configure(conn *sqlite.Conn) error {
	if err := conn.Exec("PRAGMA temp_store=MEMORY; PRAGMA journal_mode=DELETE;"); err != nil {
		return err
	}
	conn.Limit(sqlite.LIMIT_LENGTH, 1<<20)
	conn.Limit(sqlite.LIMIT_SQL_LENGTH, 1<<20)
	conn.Limit(sqlite.LIMIT_ATTACHED, 0)
	conn.HardHeapLimit(32 << 20)
	return conn.SetAuthorizer(func(action sqlite.AuthorizerActionCode, first, second, schema, inner string) sqlite.AuthorizerReturnCode {
		switch action {
		case sqlite.AUTH_ATTACH, sqlite.AUTH_DETACH, sqlite.AUTH_CREATE_VTABLE, sqlite.AUTH_DROP_VTABLE:
			return sqlite.AUTH_DENY
		case sqlite.AUTH_PRAGMA:
			switch strings.ToLower(first) {
			case "query_only", "table_info", "table_xinfo", "index_list", "index_info", "foreign_key_list", "database_list":
				return sqlite.AUTH_OK
			default:
				return sqlite.AUTH_DENY
			}
		case sqlite.AUTH_FUNCTION:
			switch strings.ToLower(second) {
			case "load_extension", "readfile", "writefile", "edit", "fts3_tokenizer":
				return sqlite.AUTH_DENY
			}
		}
		return sqlite.AUTH_OK
	})
}

type rootVFS struct {
	root     *os.Root
	database string
}

func (v *rootVFS) allowed(name string) bool {
	return name == v.database || name == v.database+"-journal"
}
func (v *rootVFS) FullPathname(name string) (string, error) {
	if !v.allowed(name) {
		return "", fs.ErrPermission
	}
	return name, nil
}
func (v *rootVFS) Access(name string, flags vfs.AccessFlag) (bool, error) {
	if name == v.database+"-wal" {
		return false, nil
	}
	if !v.allowed(name) {
		return false, fs.ErrPermission
	}
	info, err := v.root.Lstat(name)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !info.Mode().IsRegular() {
		return false, fs.ErrPermission
	}
	return true, nil
}
func (v *rootVFS) Delete(name string, syncDir bool) error {
	if !v.allowed(name) {
		return fs.ErrPermission
	}
	if err := v.root.Remove(name); err != nil {
		return err
	}
	if syncDir {
		dir, err := v.root.Open(".")
		if err != nil {
			return err
		}
		defer dir.Close()
		return dir.Sync()
	}
	return nil
}
func (v *rootVFS) Open(name string, flags vfs.OpenFlag) (vfs.File, vfs.OpenFlag, error) {
	if name == "" && flags&(vfs.OPEN_TEMP_JOURNAL|vfs.OPEN_TEMP_DB|vfs.OPEN_TRANSIENT_DB) != 0 {
		return &vfsutil.SliceFile{}, flags | vfs.OPEN_MEMORY, nil
	}
	if !v.allowed(name) || flags&(vfs.OPEN_MAIN_DB|vfs.OPEN_MAIN_JOURNAL) == 0 {
		return nil, flags, fs.ErrPermission
	}
	if flags&vfs.OPEN_MAIN_DB != 0 {
		for _, suffix := range []string{"-wal", "-shm"} {
			if _, err := v.root.Lstat(v.database + suffix); !errors.Is(err, fs.ErrNotExist) {
				return nil, flags, fs.ErrPermission
			}
		}
	}
	mode := os.O_RDONLY
	if flags&vfs.OPEN_READWRITE != 0 {
		mode = os.O_RDWR
	}
	if flags&vfs.OPEN_CREATE != 0 {
		mode |= os.O_CREATE
	}
	file, err := v.root.OpenFile(name, mode|unix.O_NOFOLLOW, 0600)
	if err != nil {
		return nil, flags, err
	}
	var stat unix.Stat_t
	if err := unix.Fstat(int(file.Fd()), &stat); err != nil || stat.Nlink != 1 {
		file.Close()
		return nil, flags, fs.ErrPermission
	}
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		file.Close()
		return nil, flags, fs.ErrPermission
	}
	if flags&vfs.OPEN_MAIN_DB != 0 {
		if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
			file.Close()
			return nil, flags, sqlite.BUSY
		}
	}
	return &rootFile{File: file, root: v.root, syncDirectory: flags&vfs.OPEN_CREATE != 0}, flags, nil
}

type rootFile struct {
	*os.File
	level         vfs.LockLevel
	root          *os.Root
	syncDirectory bool
}

func (f *rootFile) Sync(vfs.SyncFlag) error {
	if err := f.File.Sync(); err != nil {
		return err
	}
	if f.syncDirectory {
		dir, err := f.root.Open(".")
		if err != nil {
			return err
		}
		defer dir.Close()
		if err := dir.Sync(); err != nil {
			return err
		}
		f.syncDirectory = false
	}
	return nil
}
func (f *rootFile) Size() (int64, error) {
	info, err := f.Stat()
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// The main file already holds an exclusive OS lock for its entire lifetime.
func (f *rootFile) Lock(level vfs.LockLevel) error                  { f.level = level; return nil }
func (f *rootFile) Unlock(level vfs.LockLevel) error                { f.level = level; return nil }
func (f *rootFile) CheckReservedLock() (bool, error)                { return f.level >= vfs.LOCK_RESERVED, nil }
func (f *rootFile) SectorSize() int                                 { return 4096 }
func (f *rootFile) DeviceCharacteristics() vfs.DeviceCharacteristic { return 0 }
