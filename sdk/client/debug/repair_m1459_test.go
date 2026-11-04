package debug

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strconv"
	"testing"
)

func executeRepairM1459RawBytes(t *testing.T, input string) (string, error) {
	t.Helper()

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout := os.Stdout
	t.Cleanup(func() {
		os.Stdout = stdout
		writer.Close()
		reader.Close()
	})
	os.Stdout = writer

	cmd := Cmd()
	var diagnostics bytes.Buffer
	cmd.SetOut(&diagnostics)
	cmd.SetErr(&diagnostics)
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{"raw-bytes", input})
	commandErr := cmd.Execute()
	os.Stdout = stdout
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if diagnostics.Len() != 0 {
		t.Fatalf("unexpected command diagnostics: %q", diagnostics.String())
	}
	return string(output), commandErr
}

func TestRepairM_1459_UnsignedByteRange(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input string
		want  string
	}{
		{name: "zero", input: "[0]", want: "00\n"},
		{name: "127", input: "[127]", want: "7F\n"},
		{name: "128", input: "[128]", want: "80\n"},
		{name: "255", input: "[255]", want: "FF\n"},
		{name: "mixed_range", input: "[0 127 128 255]", want: "007F80FF\n"},
		{name: "advertised_example", input: "[10 21 13 255]", want: "0A150DFF\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			output, err := executeRepairM1459RawBytes(t, tc.input)
			if err != nil {
				t.Fatalf("raw-bytes %q: %v", tc.input, err)
			}
			if output != tc.want {
				t.Fatalf("raw-bytes %q emitted %q; want %q", tc.input, output, tc.want)
			}
		})
	}
}

func TestRepairM_1459_RejectInvalidBytes(t *testing.T) {
	for _, tc := range []struct {
		name    string
		input   string
		element string
		wantErr error
	}{
		{name: "negative", input: "[-1]", element: "-1", wantErr: strconv.ErrSyntax},
		{name: "overflow", input: "[256]", element: "256", wantErr: strconv.ErrRange},
		{name: "negative_after_valid", input: "[0 -1 255]", element: "-1", wantErr: strconv.ErrSyntax},
		{name: "overflow_after_valid", input: "[127 256 128]", element: "256", wantErr: strconv.ErrRange},
	} {
		t.Run(tc.name, func(t *testing.T) {
			output, err := executeRepairM1459RawBytes(t, tc.input)
			if err == nil {
				t.Fatalf("raw-bytes %q accepted an invalid byte", tc.input)
			}
			var parseErr *strconv.NumError
			if !errors.As(err, &parseErr) || parseErr.Num != tc.element || !errors.Is(err, tc.wantErr) {
				t.Fatalf("raw-bytes %q returned %v; want numeric error %v for %q", tc.input, err, tc.wantErr, tc.element)
			}
			if output != "" {
				t.Fatalf("raw-bytes %q emitted %q on refusal", tc.input, output)
			}
		})
	}
}
