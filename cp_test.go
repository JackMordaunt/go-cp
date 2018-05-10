package cp

import (
	"testing"

	"github.com/spf13/afero"

	fb "github.com/jackmordaunt/filebuilder"
)

// TestCopy_LateralCopy tests that copying laterally yields an exact match in
// the new directory.
func TestCopy_LateralCopy(t *testing.T) {
	tests := []struct {
		desc     string
		from, to string
		files    fb.Entry
		// files which exist at `to`. For testing clobber conditions.
		toClobber fb.Entry
		clobber   bool
		wantErr   bool
	}{
		{
			"noop copy",
			"from",
			"to",
			nil,
			nil,
			false,
			true,
		},
		{
			"list of files",
			"from",
			"to",
			fb.Entries([]fb.Entry{
				fb.File{Path: "foo.exe"},
				fb.File{Path: "bar.exe"},
				fb.File{Path: "foobar.exe"},
			}),
			nil,
			false,
			false,
		},
		{
			"do not clobber existing files",
			"from",
			"to",
			fb.Entries([]fb.Entry{
				fb.File{Path: "foo.exe"},
				fb.File{Path: "bar.exe"},
				fb.File{Path: "foobar.exe"},
			}),
			fb.Entries([]fb.Entry{
				fb.File{Path: "foo.exe"},
				fb.File{Path: "bar.exe"},
				fb.File{Path: "foobar.exe"},
			}),
			false,
			true,
		},
		{
			"clobber existing files",
			"from",
			"to",
			fb.Entries([]fb.Entry{
				fb.File{Path: "foo.exe"},
				fb.File{Path: "bar.exe"},
				fb.File{Path: "foobar.exe"},
			}),
			fb.Entries([]fb.Entry{
				fb.File{Path: "foo.exe"},
				fb.File{Path: "bar.exe"},
				fb.File{Path: "foobar.exe"},
			}),
			true,
			false,
		},
		{
			"directory with files",
			"from",
			"to",
			fb.Entries([]fb.Entry{
				fb.Directory{
					Path: "dir",
					Entries: []fb.Entry{
						fb.File{Path: "foo.exe"},
						fb.File{Path: "bar.exe"},
						fb.File{Path: "foobar.exe"},
					},
				},
			}),
			nil,
			false,
			false,
		},
	}
	for _, tt := range tests {
		fs := afero.NewMemMapFs()
		if _, err := fb.Build(fs, tt.from, tt.files); err != nil {
			t.Fatalf("[%s] unexpected error while building filesystem: %v",
				tt.desc, err)
		}
		if tt.toClobber != nil {
			if _, err := fb.Build(fs, tt.to, tt.toClobber); err != nil {
				t.Fatalf("[%s] unexpected error while building clobber files: %v",
					tt.desc, err)
			}
		}
		copier := Copier{
			Fs:      fs,
			Clobber: tt.clobber,
		}
		err := copier.Copy(tt.from, tt.to)
		if err != nil && !tt.wantErr {
			t.Fatalf("[%s] unexpected error while copying: %v", tt.desc, err)
		}
		if err == nil && tt.wantErr {
			t.Fatalf("[%s] want error during copy, got nil", tt.desc)
		}
		if tt.files == nil {
			continue
		}
		diff, ok, err := fb.CompareDirectories(fs, tt.from, tt.to)
		if err != nil {
			t.Fatalf("[%s] unexpected error comparing directories: %v", tt.desc, err)
		}
		if !ok {
			t.Fatalf("[%s] copy not exact: \n%v", tt.desc, diff)
		}
	}
}

// TestCopy_VerticalCopy tests that copying vertically does not end in infinite
// recursion. That is we should should be able to copy into a child directory
// or a parent directory without issue. Maybe, see TODO.
// "cp -r parent/child parent" copies the contents of child into parent.
// "cp -r parent parent/child" causes infinite recursion.
func TestCopy_VerticalCopy(t *testing.T) {
	tests := []struct {
		desc     string
		from     string
		to       string
		original fb.Entry
		expected fb.Entry
	}{
		// This test, "copy into child", will give you infinite recursion
		// using cp -r.
		// For this library however it gives inconsistent results:
		// sometimes the test passes and sometimes it fails.
		//
		// TODO(jfm): Should this usecase throw an error or should we
		// handle it?
		// Perhaps one is just asking for trouble by attempting such a
		// command.
		//
		// {
		// 	"copy into child",
		// 	// cp from from/to
		// 	"/from",
		// 	"/from/to",
		// 	fb.Entries([]fb.Entry{
		// 		fb.File{Path: "/dir/foo.exe"},
		// 		fb.File{Path: "/dir/bar.exe"},
		// 		fb.File{Path: "/dir/foobar.exe"},
		// 	}),
		// 	fb.Entries([]fb.Entry{
		// 		fb.File{Path: "/dir/foo.exe"},
		// 		fb.File{Path: "/dir/bar.exe"},
		// 		fb.File{Path: "/dir/foobar.exe"},
		// 		// New directory "to" with the original contents
		// 		// of "from".
		// 		fb.File{Path: "/to/dir/foo.exe"},
		// 		fb.File{Path: "/to/dir/bar.exe"},
		// 		fb.File{Path: "/to/dir/foobar.exe"},
		// 	}),
		// },
		{
			"copy into parent",
			"/from/child",
			"/from",
			fb.Entries([]fb.Entry{
				fb.File{Path: "/child/dir/foo.exe"},
				fb.File{Path: "/child/dir/bar.exe"},
				fb.File{Path: "/child/dir/foobar.exe"},
			}),
			fb.Entries([]fb.Entry{
				fb.File{Path: "/child/dir/foo.exe"},
				fb.File{Path: "/child/dir/bar.exe"},
				fb.File{Path: "/child/dir/foobar.exe"},
				// "from/dir" contains the original contents of
				// "from/child/dir"
				fb.File{Path: "/dir/foo.exe"},
				fb.File{Path: "/dir/bar.exe"},
				fb.File{Path: "/dir/foobar.exe"},
			}),
		},
	}
	for _, tt := range tests {
		original := afero.NewMemMapFs()
		if _, err := fb.Build(original, tt.from, tt.original); err != nil {
			t.Fatalf("[%s] unexpected error while building filesystem: %v",
				tt.desc, err)
		}
		expected := afero.NewMemMapFs()
		if _, err := fb.Build(expected, tt.from, tt.expected); err != nil {
			t.Fatalf("[%s] unexpected error while building filesystem: %v",
				tt.desc, err)
		}
		copier := Copier{
			Fs:      original,
			Clobber: true,
		}
		err := copier.Copy(tt.from, tt.to)
		if err != nil {
			t.Fatalf("[%s] unexpected error while copying: %v",
				tt.desc, err)
		}
		diff, ok, err := fb.Compare(expected, original)
		if err != nil {
			t.Fatalf("[%s] unexpected error comparing filesystems: %v",
				tt.desc, err)
		}
		if !ok {
			t.Fatalf("[%s] filesystems have these differences: \n%v",
				tt.desc, diff)
		}
	}
}
