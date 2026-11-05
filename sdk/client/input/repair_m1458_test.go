package input

import (
	"bufio"
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRepairM_1458_ExactAffirmatives(t *testing.T) {
	for _, response := range []string{"y", "Y", "yes", "yeS", "yEs", "yES", "Yes", "YeS", "YEs", "YES"} {
		for _, ending := range []string{"\n", "\r\n", ""} {
			t.Run(response+"/"+strings.ReplaceAll(strings.ReplaceAll(ending, "\r", "CR"), "\n", "LF"), func(t *testing.T) {
				var output bytes.Buffer
				reader := bufio.NewReader(strings.NewReader(" \t\u2003" + response + "\u2003\t " + ending))
				confirmed, err := GetConfirmation("Overwrite the existing key?", reader, &output)
				require.NoError(t, err)
				require.True(t, confirmed)
			})
		}
	}
}

func TestRepairM_1458_RejectPrefixLookalikes(t *testing.T) {
	for _, response := range []string{
		"yesterday", "YESTERDAY", "yellow", "yeah", "yep", "yesplease", "YESPLEASE",
		"yes!", "yes no", "y es", "y\tno", "y0", "yes\x00", "y\u200b", "no", "n", "1",
	} {
		for _, ending := range []string{"\n", "\r\n", ""} {
			t.Run(response+"/"+strings.ReplaceAll(strings.ReplaceAll(ending, "\r", "CR"), "\n", "LF"), func(t *testing.T) {
				var output bytes.Buffer
				reader := bufio.NewReader(strings.NewReader(" \t" + response + "\t " + ending))
				confirmed, err := GetConfirmation("Delete the key?", reader, &output)
				require.NoError(t, err)
				require.False(t, confirmed, "unrecognized response %q must not authorize deletion", response)
			})
		}
	}
}

func TestRepairM_1458_BlankAndEOF(t *testing.T) {
	for _, response := range []string{"\n", "\r\n", " \t\n", " \t\u2003"} {
		t.Run(response, func(t *testing.T) {
			var output bytes.Buffer
			reader := bufio.NewReader(strings.NewReader(response))
			confirmed, err := GetConfirmation("Export the private key?", reader, &output)
			require.NoError(t, err)
			require.False(t, confirmed)
			confirmed, err = GetConfirmation("Export the private key?", reader, &output)
			require.ErrorIs(t, err, io.EOF)
			require.False(t, confirmed)
		})
	}
	t.Run("empty EOF", func(t *testing.T) {
		var output bytes.Buffer
		confirmed, err := GetConfirmation("Export the private key?", bufio.NewReader(strings.NewReader("")), &output)
		require.ErrorIs(t, err, io.EOF)
		require.False(t, confirmed)
	})
}

func TestRepairM_1458_BufferedTranscript(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("yesterday\ny\nyesplease\n YeS \n\nno\nY"))
	var output bytes.Buffer
	for i, expected := range []bool{false, true, false, true, false, false, true} {
		confirmed, err := GetConfirmation("Overwrite the existing key?", reader, &output)
		require.NoError(t, err, "response %d", i)
		require.Equal(t, expected, confirmed, "response %d", i)
	}
	confirmed, err := GetConfirmation("Overwrite the existing key?", reader, &output)
	require.ErrorIs(t, err, io.EOF)
	require.False(t, confirmed)
}
