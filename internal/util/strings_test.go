package util

import (
	"testing"
	"time"

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

		{"vol substring in stem is still index", "/data/test.volcano.par2", true},
		{"vol substring in stem no dir is index", "test.volcano.par2", true},
		{"p2c stem is index (not bundle suffix)", "/data/test.p2c_backup.par2", true},

		{"malformed volume no plus is index", "/data/test.vol01.par2", true},
		{"malformed volume non-digit rhs is index", "/data/test.vol01+ab.par2", true},
		{"malformed volume non-digit lhs is index", "/data/test.volab+01.par2", true},
		{"malformed volume double plus is index", "/data/test.vol01+02+03.par2", true},

		{"canonical volume with p2c root is not index", "/data/test.p2c.vol01+02.par2", false},
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
		{"lowercase alt", "test.vol00-01.par2", true},
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

		{"vol substring in stem (not volume)", "test.volcano.par2", false},
		{"vol substring in stem with dir (not volume)", "/data/test.volcano.par2", false},
		{"vol without count", "test.vol01.par2", false},
		{"vol without start", "test.vol+01.par2", false},
		{"vol trailing plus", "test.vol01+.par2", false},
		{"vol non-digit start", "test.volab+01.par2", false},
		{"vol non-digit count", "test.vol01+ab.par2", false},
		{"vol extra plus", "test.vol01+02+03.par2", false},
		{"p2c in stem then canonical vol", "test.p2c.vol01+02.par2", true},
		{"p2c after vol marker (invalid)", "test.vol01+02.p2c.par2", false},
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
		{"lowercase bundle", "test.p2c.par2", true},
		{"uppercase par2", "test.p2c.PAR2", true},
		{"with directory", "/data/folder/test.p2c.par2", true},
		{"hidden file", ".test.p2c.par2", true},
		{"hidden file with directory", "/data/folder/.test.p2c.par2", true},
		{"misleading name", "x.p2cbackup.par2", false},

		{"plain par2 index", "test.par2", false},
		{"plain par2 volume", "test.vol00+01.par2", false},
		{"misleading par2 volume", "test.p2c.vol00+01.par2", false},
		{"misleading par2 bundle", "test.vol00+01.p2c.par2", true},
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

// Expectation: IsPar2SetMember should match only canonical members of one PAR2
// set (index, bundle, strict volumes), case-insensitively.
func Test_IsPar2SetMember_Table(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		par2Name  string
		candidate string
		expect    bool
	}{
		// index anchor
		{"index->index", "test.par2", "test.par2", true},
		{"index->bundle", "test.par2", "test.p2c.par2", true},
		{"index->volume", "test.par2", "test.vol00+01.par2", true},
		{"index->alt volume", "test.par2", "test.vol00-01.par2", true},
		{"index->volume uppercase", "test.PAR2", "TEST.VOL00+01.PAR2", true},
		{"index->wrong p2c volume", "test.par2", "test.p2c.vol10+20.par2", false},

		// bundle anchor should map to same root set
		{"bundle->bundle", "test.p2c.par2", "test.p2c.par2", true},
		{"bundle->index", "test.p2c.par2", "test.par2", true},
		{"bundle->volume", "test.p2c.par2", "test.vol10+20.par2", true},
		{"bundle->wrong p2c volume", "test.p2c.par2", "test.p2c.vol10+20.par2", false},

		// dotted roots
		{"dotted index", "test.backup.par2", "test.backup.par2", true},
		{"dotted bundle", "test.backup.par2", "test.backup.p2c.par2", true},
		{"dotted volume", "test.backup.par2", "test.backup.vol00+01.par2", true},
		{"dotted mismatch sibling", "test.backup.par2", "test.other.par2", false},
		{"short root not dotted set", "test.par2", "test.backup.par2", false},

		// malformed / non-members
		{"different base", "test.par2", "other.par2", false},
		{"partial base", "test.par2", "testing.par2", false},
		{"vol no plus", "test.par2", "test.vol01.par2", false},
		{"vol double plus", "test.par2", "test.vol00+01+02.par2", false},
		{"vol non-digit lhs", "test.par2", "test.volab+01.par2", false},
		{"vol non-digit rhs", "test.par2", "test.vol01+ab.par2", false},
		{"wrong extension", "test.par2", "test.vol00+01.txt", false},
		{"empty candidate", "test.par2", "", false},

		// dirs in inputs should not affect basename matching
		{"par2Name dir with .vol segment", "/data/.vol01/test.par2", "test.par2", true},
		{"candidate with dir", "test.par2", "/other/test.vol05+10.par2", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.expect, IsPar2SetMember(tt.par2Name, tt.candidate))
		})
	}
}

// Expectation: The function should meet the table's expectations.
func Test_isVolumeNameForRoot_Table(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		fname  string
		root   string
		sep    byte
		expect bool
	}{
		// valid plus-separated volumes
		{"basic plus", "test.vol00+01.par2", "test", '+', true},
		{"large numbers plus", "test.vol999+100.par2", "test", '+', true},
		{"single digits plus", "test.vol0+1.par2", "test", '+', true},
		{"leading zeros plus", "test.vol007+010.par2", "test", '+', true},
		{"dotted root plus", "test.backup.vol00+01.par2", "test.backup", '+', true},

		// valid minus-separated volumes
		{"basic minus", "test.vol00-01.par2", "test", '-', true},
		{"large numbers minus", "test.vol999-100.par2", "test", '-', true},
		{"dotted root minus", "test.backup.vol05-10.par2", "test.backup", '-', true},

		// separator mismatch
		{"plus name minus sep", "test.vol00+01.par2", "test", '-', false},
		{"minus name plus sep", "test.vol00-01.par2", "test", '+', false},

		// wrong root
		{"different root", "other.vol00+01.par2", "test", '+', false},
		{"longer root", "testing.vol00+01.par2", "test", '+', false},
		{"shorter root", "tes.vol00+01.par2", "test", '+', false},

		// malformed mid section
		{"no separator", "test.vol01.par2", "test", '+', false},
		{"double separator", "test.vol01+02+03.par2", "test", '+', false},
		{"leading separator", "test.vol+01.par2", "test", '+', false},
		{"trailing separator", "test.vol01+.par2", "test", '+', false},
		{"non-digit lhs", "test.volab+01.par2", "test", '+', false},
		{"non-digit rhs", "test.vol01+ab.par2", "test", '+', false},
		{"empty mid", "test.vol.par2", "test", '+', false},

		// wrong extension
		{"txt extension", "test.vol00+01.txt", "test", '+', false},
		{"no extension", "test.vol00+01", "test", '+', false},

		// not a volume at all
		{"index file", "test.par2", "test", '+', false},
		{"no vol prefix", "test.00+01.par2", "test", '+', false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.expect, isVolumeNameForRoot(tt.fname, tt.root, tt.sep))
		})
	}
}

// Expectation: The function should meet the table's expectations.
func Test_isDigits_Table(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  string
		expect bool
	}{
		{"single digit", "0", true},
		{"multiple digits", "12345", true},
		{"leading zeros", "007", true},
		{"all zeros", "000", true},
		{"large number", "9999999", true},

		{"empty string", "", false},
		{"letters only", "abc", false},
		{"mixed digits letters", "12ab", false},
		{"digits then letter", "123a", false},
		{"letter then digits", "a123", false},
		{"space in digits", "1 2", false},
		{"negative sign", "-1", false},
		{"decimal point", "1.0", false},
		{"plus sign", "+1", false},
		{"unicode digit", "١٢٣", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.expect, isDigits(tt.input))
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
