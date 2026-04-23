package util

import (
	"strings"
	"testing"
	"time"

	"github.com/desertwitch/par2cron/internal/schema"
	"github.com/stretchr/testify/require"
)

// Expectation: The function should meet the table's expectations.
func Test_IsPar2Index_Table(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		input  string
		expect bool
	}{
		{"lowercase extension", "/data/test.par2", true},
		{"uppercase extension", "/data/test.PAR2", true},
		{"mixed case extension", "/data/test.Par2", true},
		{"no directory", "test.par2", true},
		{"vol in directory path", "/data/.vol01/test.par2", true},
		{"vol+digits in directory path", "/data/vol00+01/test.PAR2", true},
		{"dot vol in directory path", "/mnt/.vol/archive.par2", true},

		{"volume file lowercase", "/data/test.vol01+02.par2", false},
		{"volume file uppercase", "/data/test.vol10+20.PAR2", false},
		{"volume file mixed case", "/data/test.VOL00+01.Par2", false},

		{"txt file", "/data/test.txt", false},
		{"par file (no 2)", "/data/test.par", false},
		{"no extension", "/data/test", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.expect, IsPar2Index(tt.input))
		})
	}
}

// Expectation: The function should meet the table's expectations.
func Test_IsPar2Volume_Table(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		input  string
		expect bool
	}{
		{"lowercase", "test.vol00+01.par2", true},
		{"uppercase extension", "test.vol25+22.PAR2", true},
		{"mixed case", "test.VOL00+01.Par2", true},
		{"with directory", "/data/test.vol10+20.par2", true},
		{"nested directory", "/data/folder/test.vol99+50.PAR2", true},
		{"large block count", "test.vol000+100.par2", true},
		{"single digit blocks", "test.vol0+1.par2", true},

		{"index lowercase", "test.par2", false},
		{"index uppercase", "test.PAR2", false},
		{"index with directory", "/data/test.par2", false},

		{"txt file", "test.txt", false},
		{"vol pattern but txt", "test.vol01+02.txt", false},
		{"no extension", "test", false},
		{"empty string", "", false},

		{"vol dir with index file", "/data/.vol01/test.par2", false},
		{"vol+digits dir with index file", "/data/vol00+01/test.PAR2", false},
		{"dot vol dir with index file", "/mnt/.vol/archive.par2", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.expect, IsPar2Volume(tt.input))
		})
	}
}

// Expectation: The function should meet the table's expectations.
func Test_IsPar2Bundle_Table(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  string
		expect bool
	}{
		{"lowercase bundle", "test" + schema.BundleExtension + schema.Par2Extension, true},
		{"uppercase par2", "test" + schema.BundleExtension + strings.ToUpper(schema.Par2Extension), true},
		{"with directory", "/data/folder/test" + schema.BundleExtension + schema.Par2Extension, true},
		{"hidden file", ".test" + schema.BundleExtension + schema.Par2Extension, true},
		{"hidden file with directory", "/data/folder/.test" + schema.BundleExtension + schema.Par2Extension, true},

		{"plain par2 index", "test" + schema.Par2Extension, false},
		{"plain par2 volume", "test.vol00+01" + schema.Par2Extension, false},
		{"txt file", "test.txt", false},
		{"no extension", "test", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tt.expect, IsPar2Bundle(tt.input))
		})
	}
}

// Expectation: The function should meet the table's expectations.
func Test_EndsWithFold_Table(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		s      string
		suffix string
		expect bool
	}{
		{"exact lowercase", "test.par2", ".par2", true},
		{"uppercase input", "test.PAR2", ".par2", true},
		{"mixed case input", "test.Par2", ".par2", true},
		{"mixed case both", "test.pAr2", ".PAR2", true},
		{"with directory path", "/data/folder/test.PAR2", ".par2", true},
		{"suffix equals string", ".par2", ".par2", true},

		{"partial suffix", "test.par", ".par2", false},
		{"different extension", "test.txt", ".par2", false},
		{"no extension", "test", ".par2", false},
		{"empty string", "", ".par2", false},
		{"extension in directory", "", "/a/.par2/b", false},

		{"shorter than suffix", ".pa", ".par2", false},
		{"single char", "a", ".par2", false},

		{"empty suffix", "test.par2", "", true},
		{"both empty", "", "", true},
		{"suffix longer than string", "ab", "abc", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tt.expect, EndsWithFold(tt.s, tt.suffix))
		})
	}
}

// Expectation: The function should meet the table's expectations.
func Test_TrimSuffixFold_Table(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		s      string
		suffix string
		expect string
	}{
		{"exact lowercase", "test.par2", ".par2", "test"},
		{"uppercase input", "test.PAR2", ".par2", "test"},
		{"uppercase suffix", "test.par2", ".PAR2", "test"},
		{"mixed case input", "test.Par2", ".par2", "test"},
		{"mixed case both", "test.pAr2", ".PAR2", "test"},
		{"with directory path", "/data/folder/test.PAR2", ".par2", "/data/folder/test"},

		{"no match", "test.txt", ".par2", "test.txt"},
		{"partial match", "test.par", ".par2", "test.par"},
		{"suffix longer than string", "ab", "abc", "ab"},

		{"empty suffix", "test.par2", "", "test.par2"},
		{"both empty", "", "", ""},
		{"empty string nonempty suffix", "", ".par2", ""},

		{"suffix equals string", ".par2", ".par2", ""},
		{"suffix equals string case fold", ".PAR2", ".par2", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.expect, TrimSuffixFold(tt.s, tt.suffix))
		})
	}
}

// Expectation: The duration should be formatted to string with success.
func Test_FmtDur_Success(t *testing.T) {
	t.Parallel()

	result := FmtDur(90 * time.Minute)

	require.NotEmpty(t, result)
	require.NotEqual(t, "?", result)
}

// Expectation: The duration should be formatted to string with success.
func Test_FmtDur_Negative_Success(t *testing.T) {
	t.Parallel()

	result := FmtDur(-1)

	require.NotEmpty(t, result)
	require.NotEqual(t, "?", result)
}

// Expectation: The duration should be formatted to string with success.
func Test_FmtDur_ZeroDuration_Success(t *testing.T) {
	t.Parallel()

	result := FmtDur(0)

	require.NotEmpty(t, result)
	require.NotEqual(t, "?", result)
}

// Expectation: The function should meet the table's expectations.
func Test_IsGlobRecursive_Table(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		pattern string
		expect  bool
	}{
		{"contains forward slash", "dir/file", true},
		{"contains double star", "dir**file", true},
		{"contains both slash and double star", "dir/**/file", true},
		{"starts with double star", "**/file", true},
		{"ends with double star", "dir/**", true},
		{"only forward slash", "/", true},
		{"only double star", "**", true},
		{"simple glob star", "*.txt", false},
		{"question mark glob", "file?.txt", false},
		{"no special chars", "file.txt", false},
		{"empty string", "", false},
		{"single star no slash", "dir*file", false},
		{"bracket glob", "[abc].txt", false},
		{"triple star", "***", true},
		{"multiple slashes", "a/b/c", true},
		{"backslash not forward", "dir\\file", false},
		{"star before extension", "*.par2", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.expect, IsGlobRecursive(tt.pattern))
		})
	}
}
