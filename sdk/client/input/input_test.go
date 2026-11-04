package input

import (
	"bufio"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type fakeReader struct {
	fnc func(p []byte) (int, error)
}

func (f fakeReader) Read(p []byte) (int, error) {
	return f.fnc(p)
}

var _ io.Reader = fakeReader{}

func TestGetConfirmation(t *testing.T) {
	for _, tc := range []struct {
		name      string
		response  string
		confirmed bool
		err       error
	}{
		{name: "short lowercase", response: "y\n", confirmed: true},
		{name: "short uppercase", response: "Y\n", confirmed: true},
		{name: "full lowercase", response: "yes\n", confirmed: true},
		{name: "full uppercase", response: "YES\n", confirmed: true},
		{name: "mixed case", response: "YeS\n", confirmed: true},
		{name: "surrounding whitespace", response: " \tYes\t \r\n", confirmed: true},
		{name: "prefix word", response: "yesterday\n"},
		{name: "full word prefix", response: "yesplease\n"},
		{name: "negative", response: "no\n"},
		{name: "blank", response: "\n"},
		{name: "whitespace", response: " \t\r\n"},
		{name: "affirmative at EOF", response: "yes", confirmed: true},
		{name: "prefix at EOF", response: "yellow"},
		{name: "empty EOF", err: io.EOF},
	} {
		t.Run(tc.name, func(t *testing.T) {
			confirmed, err := GetConfirmation("Confirm", bufio.NewReader(strings.NewReader(tc.response)), io.Discard)
			if tc.err != nil {
				require.ErrorIs(t, err, tc.err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tc.confirmed, confirmed)
		})
	}
}

func TestReadLineFromBuf(t *testing.T) {
	var fr fakeReader

	t.Run("it correctly returns the password when reader returns EOF", func(t *testing.T) {
		fr.fnc = func(p []byte) (int, error) {
			return copy(p, []byte("hello")), io.EOF
		}
		buf := bufio.NewReader(fr)

		pass, err := readLineFromBuf(buf)
		require.NoError(t, err)
		require.Equal(t, "hello", pass)
	})

	t.Run("it returns EOF if reader has been exhausted", func(t *testing.T) {
		fr.fnc = func(p []byte) (int, error) {
			return 0, io.EOF
		}
		buf := bufio.NewReader(fr)

		_, err := readLineFromBuf(buf)
		require.ErrorIs(t, err, io.EOF)
	})

	t.Run("it returns the error if it's not EOF regardles if it read something or not", func(t *testing.T) {
		expectedErr := errors.New("oh no")
		fr.fnc = func(p []byte) (int, error) {
			return copy(p, []byte("hello")), expectedErr
		}
		buf := bufio.NewReader(fr)

		_, err := readLineFromBuf(buf)
		require.ErrorIs(t, err, expectedErr)
	})

}
