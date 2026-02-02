package create

import (
	"fmt"
	"io"
	"testing"

	"github.com/desertwitch/par2cron/internal/flags"
	"github.com/desertwitch/par2cron/internal/logging"
	"github.com/desertwitch/par2cron/internal/schema"
	"github.com/desertwitch/par2cron/internal/testutil"
	"github.com/desertwitch/par2cron/internal/util"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

// Expectation: The default configuration should be returned.
func Test_NewMarkerConfig_Success(t *testing.T) {
	t.Parallel()

	args := Options{
		Par2Args: []string{"-r10", "-n3"},
		Par2Glob: "*.mp4",
	}
	require.NoError(t, args.Par2Mode.Set(schema.CreateFileMode))

	cfg := NewMarkerConfig("/data/testfolder/_par2cron", args)

	require.Equal(t, "testfolder"+schema.Par2Extension, *cfg.Par2Name)
	require.Equal(t, args.Par2Args, *cfg.Par2Args)
	require.Equal(t, args.Par2Glob, *cfg.Par2Glob)
	require.Equal(t, args.Par2Mode.Value, cfg.Par2Mode.Value)
	require.False(t, *cfg.Par2Verify)
	require.False(t, *cfg.HideFiles)
}

// Expectation: A non-nil configuration should be returned.
func Test_Service_parseMarkerFile_ValidMarker_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/"+createMarkerPathPrefix, []byte(""), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("debug")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	args := Options{Par2Args: []string{"-r10"}}
	cfg, err := prog.parseMarkerFile("/data/folder/"+createMarkerPathPrefix, args)

	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, "folder"+schema.Par2Extension, *cfg.Par2Name)
	require.Contains(t, logBuf.String(), "Found marker file")
}

// Expectation: The YAML configuration should be parsed.
func Test_Service_parseMarkerFile_WithYAMLConfig_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	yamlContent := `name: "custom.par2"
args: ["-r15", "-n5"]
glob: "*.txt"
mode: "file"
verify: true
hidden: true`
	require.NoError(t, afero.WriteFile(fs, "/data/folder/"+createMarkerPathPrefix, []byte(yamlContent), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("debug")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	args := Options{Par2Args: []string{"-r10"}}
	cfg, err := prog.parseMarkerFile("/data/folder/"+createMarkerPathPrefix, args)

	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, "custom"+schema.Par2Extension, *cfg.Par2Name)
	require.Equal(t, []string{"-r15", "-n5"}, *cfg.Par2Args)
	require.Equal(t, "*.txt", *cfg.Par2Glob)
	require.Equal(t, "file", cfg.Par2Mode.Value)
	require.True(t, *cfg.Par2Verify)
	require.True(t, *cfg.HideFiles)
}

// Expectation: The YAML configuration should reject an unknown mode.
func Test_Service_parseMarkerFile_WithYAMLConfig_UnknownMode_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	yamlContent := `name: "custom.par2"
args: ["-r15", "-n5"]
glob: "*.txt"
mode: "filez"`
	require.NoError(t, afero.WriteFile(fs, "/data/folder/_par2cron", []byte(yamlContent), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("debug")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	args := Options{Par2Args: []string{"-r10"}}
	cfg, err := prog.parseMarkerFile("/data/folder/_par2cron", args)

	require.Error(t, err)
	require.Nil(t, cfg)
}

// Expectation: An error should be returned when failing to read the marker file.
func Test_Service_parseMarkerFile_NotExist_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("debug")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	args := Options{Par2Args: []string{"-r10"}}
	cfg, err := prog.parseMarkerFile("/data/folder/"+createMarkerPathPrefix, args)

	require.Error(t, err)
	require.Nil(t, cfg)
}

// Expectation: The default configuration should be applied.
func Test_Service_parseMarkerFilename_NoSuffix_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("debug")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	cfg := &MarkerConfig{
		Par2Name: util.Ptr("test" + schema.Par2Extension),
		Par2Args: &[]string{"-r10"},
		Par2Glob: util.Ptr("*"),
	}

	prog.parseMarkerFilename("/data/_par2cron", cfg)

	require.Equal(t, []string{"-r10"}, *cfg.Par2Args)
}

// Expectation: The arguments from the filename should be applied.
func Test_Service_parseMarkerFilename_WithFlags_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("debug")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	cfg := &MarkerConfig{
		Par2Name: util.Ptr("test" + schema.Par2Extension),
		Par2Args: &[]string{"-r10", "-n3"},
		Par2Glob: util.Ptr("*"),
	}

	fname := fmt.Sprintf("%s%sr20%sn5%sqqq",
		createMarkerPathPrefix, createMarkerPathSeparator, createMarkerPathSeparator, createMarkerPathSeparator)
	prog.parseMarkerFilename("/data/"+fname, cfg)

	require.Contains(t, logBuf.String(), "Parsed setting from marker filename")
	require.Equal(t, "-r20", (*cfg.Par2Args)[0])
	require.Equal(t, "-n5", (*cfg.Par2Args)[1])
	require.Equal(t, "-qqq", (*cfg.Par2Args)[2])
}

// Expectation: The arguments from the filename should be applied without duplicates.
func Test_Service_parseMarkerFilename_WithDuplicateFlags_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("debug")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	cfg := &MarkerConfig{
		Par2Name: util.Ptr("test" + schema.Par2Extension),
		Par2Args: &[]string{"-r10", "-n3"},
		Par2Glob: util.Ptr("*"),
	}

	fname := fmt.Sprintf("%s%sr20%sn5%sqqq%sqq",
		createMarkerPathPrefix, createMarkerPathSeparator,
		createMarkerPathSeparator, createMarkerPathSeparator, createMarkerPathSeparator)
	prog.parseMarkerFilename("/data/"+fname, cfg)

	require.Contains(t, logBuf.String(), "Parsed setting from marker filename")
	require.Equal(t, "-r20", (*cfg.Par2Args)[0])
	require.Equal(t, "-n5", (*cfg.Par2Args)[1])
	require.Equal(t, "-qqq", (*cfg.Par2Args)[2])
}

// Expectation: An error should be returned on failure to read the marker file.
func Test_Service_parseMarkerContent_FailedToRead_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	cfg := &MarkerConfig{
		Par2Name: util.Ptr("test" + schema.Par2Extension),
		Par2Args: &[]string{"-r10"},
		Par2Glob: util.Ptr("*"),
	}

	err := prog.parseMarkerContent("/data/folder/_par2cron", cfg)

	require.ErrorContains(t, err, "failed to read")
}

// Expectation: An error should be returned on invalid YAML contents.
func Test_Service_parseMarkerContent_InvalidYAML_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/_par2cron", []byte("invalid yaml {]"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	cfg := &MarkerConfig{
		Par2Name: util.Ptr("test" + schema.Par2Extension),
		Par2Args: &[]string{"-r10"},
		Par2Glob: util.Ptr("*"),
	}

	err := prog.parseMarkerContent("/data/folder/_par2cron", cfg)

	require.ErrorContains(t, err, "failed to decode")
}

// Expectation: The extension should be added when none was contained.
func Test_Service_parseMarkerContent_NameWithoutExtension_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	yamlContent := `name: "custom"`
	require.NoError(t, afero.WriteFile(fs, "/data/folder/_par2cron", []byte(yamlContent), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("debug")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	cfg := &MarkerConfig{
		Par2Name: util.Ptr("test" + schema.Par2Extension),
		Par2Args: &[]string{"-r10"},
		Par2Glob: util.Ptr("*"),
	}

	err := prog.parseMarkerContent("/data/folder/_par2cron", cfg)

	require.NoError(t, err)
	require.Equal(t, "custom"+schema.Par2Extension, *cfg.Par2Name)
}

// Expectation: The replacement should work according to expectation.
func Test_Service_modifyOrAddArgument_ReplaceSpaceSeparated_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("debug")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	cfg := &MarkerConfig{
		Par2Args: &[]string{"-r 10", "-n3"},
	}

	prog.modifyOrAddArgument(cfg, "-r", "20", "/data/_par2cron")

	require.Len(t, *cfg.Par2Args, 2)
	require.Equal(t, "-r 20", (*cfg.Par2Args)[0])
	require.Equal(t, "-n3", (*cfg.Par2Args)[1])
}

// Expectation: The replacement should work according to expectation.
func Test_Service_modifyOrAddArgument_ReplaceEqualSeparated_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("debug")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	cfg := &MarkerConfig{
		Par2Args: &[]string{"-r=10", "-n3"},
	}

	prog.modifyOrAddArgument(cfg, "-r", "20", "/data/_par2cron")

	require.Len(t, *cfg.Par2Args, 2)
	require.Equal(t, "-r=20", (*cfg.Par2Args)[0])
	require.Equal(t, "-n3", (*cfg.Par2Args)[1])
}

// Expectation: The replacement should work according to expectation.
func Test_Service_modifyOrAddArgument_ReplaceNoSpace_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("debug")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	cfg := &MarkerConfig{
		Par2Args: &[]string{"-r10", "-n3"},
	}

	prog.modifyOrAddArgument(cfg, "-r", "20", "/data/_par2cron")

	require.Len(t, *cfg.Par2Args, 2)
	require.Equal(t, "-r20", (*cfg.Par2Args)[0])
	require.Equal(t, "-n3", (*cfg.Par2Args)[1])
}

// Expectation: The replacement should work according to expectation.
func Test_Service_modifyOrAddArgument_ReplaceNoValue_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("debug")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	cfg := &MarkerConfig{
		Par2Args: &[]string{"-q", "-n3"},
	}

	prog.modifyOrAddArgument(cfg, "-q", "qq", "/data/_par2cron")

	require.Len(t, *cfg.Par2Args, 2)
	require.Equal(t, "-qqq", (*cfg.Par2Args)[0])
	require.Equal(t, "-n3", (*cfg.Par2Args)[1])
}

// Expectation: The replacement should work according to expectation.
func Test_Service_modifyOrAddArgument_AddNewElement_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("debug")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	cfg := &MarkerConfig{
		Par2Args: &[]string{"-n3"},
	}

	prog.modifyOrAddArgument(cfg, "-r", "20", "/data/_par2cron")

	require.Len(t, *cfg.Par2Args, 2)
	require.Equal(t, "-n3", (*cfg.Par2Args)[0])
	require.Equal(t, "-r20", (*cfg.Par2Args)[1])
}

// Expectation: The replacement should work according to expectation.
func Test_Service_modifyOrAddArgument_ReplaceNextElement_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("debug")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	cfg := &MarkerConfig{
		Par2Args: &[]string{"-r", "10", "-n3"},
	}

	prog.modifyOrAddArgument(cfg, "-r", "20", "/data/_par2cron")

	require.Len(t, *cfg.Par2Args, 3)
	require.Equal(t, "-r", (*cfg.Par2Args)[0])
	require.Equal(t, "20", (*cfg.Par2Args)[1])
	require.Equal(t, "-n3", (*cfg.Par2Args)[2])
}

// Expectation: The mode should be set to recursive when -R is in args but mode is not recursive.
func Test_Service_considerRecursiveMarker_HasRArgButNotRecursiveMode_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("debug")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	cfg := &MarkerConfig{
		Par2Args: &[]string{"-r10", "-R"},
		Par2Mode: &flags.CreateMode{},
	}
	require.NoError(t, cfg.Par2Mode.Set(schema.CreateFileMode))

	prog.considerRecursiveMarker("/data/folder/_par2cron", cfg)

	require.Equal(t, schema.CreateRecursiveMode, cfg.Par2Mode.Value)
	require.Contains(t, logBuf.String(), "Assuming recursive mode due to set par2 argument -R")
}

// Expectation: The -R argument should be added when mode is recursive but -R is not in args.
func Test_Service_considerRecursiveMarker_RecursiveModeButNoRArg_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("debug")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	cfg := &MarkerConfig{
		Par2Args: &[]string{"-r10", "-n3"},
		Par2Mode: &flags.CreateMode{},
	}
	require.NoError(t, cfg.Par2Mode.Set(schema.CreateRecursiveMode))

	prog.considerRecursiveMarker("/data/folder/_par2cron", cfg)

	require.Equal(t, schema.CreateRecursiveMode, cfg.Par2Mode.Value)
	require.Len(t, *cfg.Par2Args, 3)
	require.Equal(t, "-r10", (*cfg.Par2Args)[0])
	require.Equal(t, "-n3", (*cfg.Par2Args)[1])
	require.Equal(t, "-R", (*cfg.Par2Args)[2])
	require.Contains(t, logBuf.String(), "Adding -R to par2 argument slice (due to set recursive mode)")
}

// Expectation: No changes should be made when mode is recursive and -R is already present.
func Test_Service_considerRecursiveMarker_RecursiveModeWithRArg_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("debug")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	cfg := &MarkerConfig{
		Par2Args: &[]string{"-r10", "-R"},
		Par2Mode: &flags.CreateMode{},
	}
	require.NoError(t, cfg.Par2Mode.Set(schema.CreateRecursiveMode))

	prog.considerRecursiveMarker("/data/folder/_par2cron", cfg)

	require.Equal(t, schema.CreateRecursiveMode, cfg.Par2Mode.Value)
	require.Len(t, *cfg.Par2Args, 2)
	require.Equal(t, "-r10", (*cfg.Par2Args)[0])
	require.Equal(t, "-R", (*cfg.Par2Args)[1])
}

// Expectation: No changes should be made when mode is file and -R is not present.
func Test_Service_considerRecursiveMarker_FileModeWithoutRArg_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("debug")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{})

	cfg := &MarkerConfig{
		Par2Args: &[]string{"-r10", "-n3"},
		Par2Mode: &flags.CreateMode{},
	}
	require.NoError(t, cfg.Par2Mode.Set(schema.CreateFileMode))

	prog.considerRecursiveMarker("/data/folder/_par2cron", cfg)

	require.Equal(t, schema.CreateFileMode, cfg.Par2Mode.Value)
	require.Len(t, *cfg.Par2Args, 2)
	require.Equal(t, "-r10", (*cfg.Par2Args)[0])
	require.Equal(t, "-n3", (*cfg.Par2Args)[1])
	require.NotContains(t, *cfg.Par2Args, "-R")
}
