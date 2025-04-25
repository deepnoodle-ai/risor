package os

import (
	"context"
	"os"
	"path/filepath"
)

var _ OS = (*SimpleOS)(nil)

type SimpleOS struct {
	ctx  context.Context
	args []string
}

func NewSimpleOS(ctx context.Context) *SimpleOS {
	sos := &SimpleOS{ctx: ctx}
	sos.args = globalScriptargs
	return sos
}

func (osObj *SimpleOS) Args() []string {
	return osObj.args
}

func (osObj *SimpleOS) Chdir(dir string) error {
	return os.Chdir(dir)
}

func (osObj *SimpleOS) Create(name string) (File, error) {
	return os.Create(name)
}

func (osObj *SimpleOS) Environ() []string {
	return os.Environ()
}

func (osObj *SimpleOS) Exit(code int) {
	os.Exit(code)
}

func (osObj *SimpleOS) Getenv(key string) string {
	return os.Getenv(key)
}

func (osObj *SimpleOS) Getpid() int {
	return os.Getpid()
}

func (osObj *SimpleOS) Getuid() int {
	return os.Getuid()
}

func (osObj *SimpleOS) Getwd() (string, error) {
	return os.Getwd()
}

func (osObj *SimpleOS) Hostname() (string, error) {
	return os.Hostname()
}

func (osObj *SimpleOS) LookupEnv(key string) (string, bool) {
	return os.LookupEnv(key)
}

func (osObj *SimpleOS) Mkdir(name string, perm FileMode) error {
	return os.Mkdir(name, perm)
}

func (osObj *SimpleOS) MkdirAll(path string, perm FileMode) error {
	return os.MkdirAll(path, perm)
}

func (osObj *SimpleOS) MkdirTemp(dir, pattern string) (string, error) {
	return os.MkdirTemp(dir, pattern)
}

func (osObj *SimpleOS) Open(name string) (File, error) {
	return os.Open(name)
}

func (osObj *SimpleOS) OpenFile(name string, flag int, perm FileMode) (File, error) {
	return os.OpenFile(name, flag, perm)
}

func (osObj *SimpleOS) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(name)
}

func (osObj *SimpleOS) Remove(name string) error {
	return os.Remove(name)
}

func (osObj *SimpleOS) RemoveAll(path string) error {
	return os.RemoveAll(path)
}

func (osObj *SimpleOS) Rename(oldpath, newpath string) error {
	return os.Rename(oldpath, newpath)
}

func (osObj *SimpleOS) Setenv(key, value string) error {
	return os.Setenv(key, value)
}

func (osObj *SimpleOS) Stat(name string) (os.FileInfo, error) {
	return os.Stat(name)
}

func (osObj *SimpleOS) Symlink(oldname, newname string) error {
	return os.Symlink(oldname, newname)
}

func (osObj *SimpleOS) TempDir() string {
	return os.TempDir()
}

func (osObj *SimpleOS) Unsetenv(key string) error {
	return os.Unsetenv(key)
}

func (osObj *SimpleOS) UserCacheDir() (string, error) {
	return os.UserCacheDir()
}

func (osObj *SimpleOS) UserConfigDir() (string, error) {
	return os.UserConfigDir()
}

func (osObj *SimpleOS) UserHomeDir() (string, error) {
	return os.UserHomeDir()
}

func (osObj *SimpleOS) WriteFile(name string, data []byte, perm FileMode) error {
	return os.WriteFile(name, data, perm)
}

func (osObj *SimpleOS) Stdin() File {
	return os.Stdin
}

func (osObj *SimpleOS) Stdout() File {
	return os.Stdout
}

func (osObj *SimpleOS) ReadDir(name string) ([]DirEntry, error) {
	results, err := os.ReadDir(name)
	if err != nil {
		return nil, err
	}
	entries := make([]DirEntry, 0, len(results))
	for _, result := range results {
		entries = append(entries, &DirEntryWrapper{result})
	}
	return entries, nil
}

func (osObj *SimpleOS) WalkDir(root string, fn WalkDirFunc) error {
	return filepath.WalkDir(root, fn)
}

func (osObj *SimpleOS) PathSeparator() rune {
	return os.PathSeparator
}

func (osObj *SimpleOS) PathListSeparator() rune {
	return os.PathListSeparator
}

func (osObj *SimpleOS) CurrentUser() (User, error) {
	return Current()
}

func (osObj *SimpleOS) LookupUser(name string) (User, error) {
	return LookupUser(name)
}

func (osObj *SimpleOS) LookupUid(uid string) (User, error) {
	return LookupUid(uid)
}

func (osObj *SimpleOS) LookupGroup(name string) (Group, error) {
	return LookupGroup(name)
}

func (osObj *SimpleOS) LookupGid(gid string) (Group, error) {
	return LookupGid(gid)
}
