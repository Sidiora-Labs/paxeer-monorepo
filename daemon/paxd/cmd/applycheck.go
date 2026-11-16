package cmd

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/flags"
	"github.com/spf13/cobra"
)

// ApplyCheckCmd compares transaction execution on a marked copy of a stopped home.
func ApplyCheckCmd(defaultHome string) *cobra.Command {
	return newApplyCheckCmd(defaultHome, os.Exit)
}

func newApplyCheckCmd(defaultHome string, exit func(int)) *cobra.Command {
	var home, input, blockTime, output string
	var standInReceiptKey bool
	var transactions []string
	finish := func(cmd *cobra.Command, code int, err error) {
		if err != nil {
			fmt.Fprintln(cmd.ErrOrStderr(), err)
		}
		if code != 0 {
			exit(code)
		}
	}
	command := &cobra.Command{
		Use: "applycheck", Short: "Compare transaction execution on a copy of a stopped node home",
		SilenceUsage: true, SilenceErrors: true,
		// This offline command reads its own configuration; the server pre-run
		// creates configuration files and therefore must not run over this home.
		PersistentPreRunE: func(*cobra.Command, []string) error { return nil },
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) != 0 {
				finish(cmd, 2, errors.New("applycheck accepts flags only"))
				return
			}
			code, err := executeApplyCheck(cmd, home, input, transactions, blockTime, output, standInReceiptKey)
			finish(cmd, code, err)
		},
	}
	command.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error { finish(cmd, 2, err); return err })
	command.Flags().StringVar(&home, flags.FlagHome, defaultHome, "Copy of a stopped node home containing applycheck-copy")
	command.Flags().StringVar(&input, "tx-file", "", "File containing one signed Ethereum hex or chain base64 transaction per line")
	command.Flags().StringArrayVar(&transactions, "tx", nil, "Signed Ethereum hex or chain base64 transaction (repeatable; appended after the file)")
	command.Flags().StringVar(&blockTime, "block-time", "", "Block time in RFC3339 format (default: last block time plus one second)")
	command.Flags().StringVar(&output, "output", "", "Write the JSON report outside the copied home (default: stdout)")
	command.Flags().BoolVar(&standInReceiptKey, "stand-in-receipt-key", false, "Use an in-memory receipt key instead of the configured receipt and consensus key files")
	return command
}

func executeApplyCheck(cmd *cobra.Command, home, input string, direct []string, blockTime, output string, standInReceiptKey bool) (int, error) {
	var txs [][]byte
	appendTransaction := func(line string) error {
		if strings.TrimSpace(line) == "" {
			return nil
		}
		tx, err := app.DecodeApplyCheckTransaction(line)
		if err != nil {
			return fmt.Errorf("transaction %d: %w", len(txs)+1, err)
		}
		txs = append(txs, tx)
		return nil
	}
	if input != "" {
		file, err := os.Open(filepath.Clean(input))
		if err != nil {
			return 2, err
		}
		defer file.Close()
		scanner := bufio.NewScanner(file)
		scanner.Buffer(make([]byte, 64*1024), 64*1024*1024)
		for scanner.Scan() {
			if err := appendTransaction(scanner.Text()); err != nil {
				return 2, err
			}
		}
		if err := scanner.Err(); err != nil {
			return 2, err
		}
	}
	for _, tx := range direct {
		if err := appendTransaction(tx); err != nil {
			return 2, err
		}
	}
	if len(txs) == 0 {
		return 2, errors.New("applycheck requires at least one transaction")
	}
	var at time.Time
	if blockTime != "" {
		var err error
		at, err = time.Parse(time.RFC3339Nano, blockTime)
		if err != nil {
			return 2, errors.New("block-time must be an RFC3339 timestamp")
		}
	}
	if output != "" {
		if err := checkApplyCheckOutput(home, output); err != nil {
			return 2, err
		}
	}
	applyCheck := sequencer.ApplyCheck{}
	applyOptions := app.ApplyCheckOptions{}
	if standInReceiptKey {
		key, err := sequencer.NewApplyCheckReceiptKey()
		if err != nil {
			return 2, err
		}
		applyCheck.ReceiptKey = key
		applyOptions.StandInReceiptKey = true
	}
	report, err := app.ApplyCheckHomeWithOptions(cmd.Context(), home, txs, at, applyCheck, applyOptions)
	if err != nil {
		return 2, err
	}
	var writer io.Writer = cmd.OutOrStdout()
	var reportFile *os.File
	if output != "" {
		file, err := os.CreateTemp(filepath.Dir(output), ".applycheck-report-*")
		if err != nil {
			return 2, err
		}
		defer file.Close()
		defer os.Remove(file.Name())
		reportFile = file
		writer = file
	}
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		return 2, err
	}
	if reportFile != nil {
		if err := reportFile.Close(); err != nil {
			return 2, err
		}
		if err := os.Rename(reportFile.Name(), output); err != nil {
			return 2, err
		}
	}
	if report.Comparison.Equal {
		return 0, nil
	}
	return 1, nil
}

func checkApplyCheckOutput(home, output string) error {
	home, err := filepath.EvalSymlinks(home)
	if err != nil {
		return err
	}
	home, err = filepath.Abs(home)
	if err != nil {
		return err
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(output))
	if err != nil {
		return err
	}
	destination := filepath.Join(parent, filepath.Base(output))
	if _, err := os.Lstat(destination); err == nil {
		destination, err = filepath.EvalSymlinks(destination)
		if err != nil {
			return err
		}
	}
	destination, err = filepath.Abs(destination)
	if err != nil {
		return err
	}
	relative, err := filepath.Rel(home, destination)
	if err != nil {
		return err
	}
	if relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return errors.New("report output must be outside the copied home")
	}
	return nil
}
