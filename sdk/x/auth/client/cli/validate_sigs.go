package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/flags"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/tx"
	cryptotypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	txtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/tx"
	authclient "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/client"
	authsigning "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/signing"
)

func GetValidateSignaturesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate-signatures [file]",
		Short: "validate transactions signatures",
		Long: `Print the addresses that must sign the transaction, those who have already
signed it, and make sure that signatures are in the correct order.

The command would check whether all required signers have signed the transactions, whether
the signatures were collected in the right order, and if the signature is valid over the
given transaction. If the --offline flag is also set, signature validation over the
transaction will be not be performed as that will require RPC communication with a full node.
`,
		PreRun: preSignCmd,
		RunE:   makeValidateSignaturesCmd(),
		Args:   cobra.ExactArgs(1),
	}

	cmd.Flags().String(flags.FlagChainID, "", "The network chain ID")
	flags.AddTxFlagsToCmd(cmd)

	return cmd
}

func makeValidateSignaturesCmd() func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		clientCtx, err := client.GetClientTxContext(cmd)
		if err != nil {
			return err
		}
		clientCtx, txBldr, stdTx, err := readTxAndInitContexts(clientCtx, cmd, args[0])
		if err != nil {
			return err
		}

		if !printAndValidateSigs(cmd, clientCtx, txBldr.ChainID(), stdTx, clientCtx.Offline) {
			return fmt.Errorf("signatures validation failed")
		}

		return nil
	}
}

// printAndValidateSigs will validate the signatures of a given transaction over its
// expected signers. In addition, if offline has not been supplied, the signature is
// verified over the transaction sign bytes. Returns false if the validation fails.
func printAndValidateSigs(
	cmd *cobra.Command, clientCtx client.Context, chainID string, tx sdk.Tx, offline bool,
) bool {
	sigTx, ok := tx.(authsigning.SigVerifiableTx)
	if !ok {
		cmd.PrintErrln("transaction does not support signature validation")
		return false
	}
	signModeHandler := clientCtx.TxConfig.SignModeHandler()

	cmd.Println("Signers:")
	signers := sigTx.GetSigners()
	for i, signer := range signers {
		cmd.Printf("  %v: %v\n", i, signer.String())
	}

	success := true
	// Check protobuf signature shapes before the conversion indexes nested
	// signatures. Malformed operator input must be a validation error.
	if protoTx, ok := tx.(interface{ GetProtoTx() *txtypes.Tx }); ok {
		parsed := protoTx.GetProtoTx()
		if parsed.AuthInfo == nil || len(parsed.AuthInfo.SignerInfos) != len(parsed.Signatures) {
			cmd.PrintErrln("signature count does not match signer info count")
			return false
		}
		for i, info := range parsed.AuthInfo.SignerInfos {
			if info == nil {
				cmd.PrintErrf("missing signer info at index %d\n", i)
				return false
			}
			if err := validateSignatureModeInfo(info.ModeInfo, parsed.Signatures[i]); err != nil {
				cmd.PrintErrf("invalid signature %d: %s\n", i, err)
				return false
			}
		}
	}
	sigs, err := sigTx.GetSignaturesV2()
	if err != nil {
		cmd.PrintErrf("failed to convert signatures: %s\n", err)
		return false
	}
	cmd.Println("")
	cmd.Println("Signatures:")

	if len(sigs) != len(signers) {
		cmd.PrintErrf("signature count %d does not match required signer count %d\n", len(sigs), len(signers))
		success = false
	}

	for i, sig := range sigs {
		var (
			pubKey         = sig.PubKey
			multiSigHeader string
			multiSigMsg    string
			sigSanity      = "OK"
		)
		if pubKey == nil {
			if offline || i >= len(signers) {
				cmd.PrintErrf("missing public key for signature %d; online account lookup is required\n", i)
				return false
			}
			if clientCtx.AccountRetriever == nil {
				cmd.PrintErrln("account retriever is required to resolve the missing public key")
				return false
			}
			account, err := clientCtx.AccountRetriever.GetAccount(clientCtx, signers[i])
			if err != nil {
				cmd.PrintErrf("failed to resolve public key for signer %s: %s\n", signers[i], err)
				return false
			}
			if account == nil || account.GetPubKey() == nil {
				cmd.PrintErrf("no public key established for signer %s\n", signers[i])
				return false
			}
			pubKey = account.GetPubKey()
		}
		sigAddr := sdk.AccAddress(pubKey.Address())

		if i >= len(signers) || !sigAddr.Equals(signers[i]) {
			sigSanity = "ERROR: signature does not match its respective signer"
			success = false
		}

		// validate the actual signature over the transaction bytes since we can
		// reach out to a full node to query accounts.
		if !offline && success {
			if clientCtx.AccountRetriever == nil {
				cmd.PrintErrln("account retriever is required for online signature validation")
				return false
			}
			accNum, accSeq, err := clientCtx.AccountRetriever.GetAccountNumberSequence(clientCtx, sigAddr)
			if err != nil {
				cmd.PrintErrf("failed to get account: %s\n", sigAddr)
				return false
			}

			signingData := authsigning.SignerData{
				ChainID:       chainID,
				AccountNumber: accNum,
				Sequence:      accSeq,
			}
			err = authsigning.VerifySignature(pubKey, signingData, sig.Data, signModeHandler, sigTx)
			if err != nil {
				cmd.PrintErrf("invalid signature %d: %s\n", i, err)
				return false
			}
		}

		cmd.Printf("  %d: %s\t\t\t[%s]%s%s\n", i, sigAddr.String(), sigSanity, multiSigHeader, multiSigMsg)
	}

	cmd.Println("")

	return success
}

func validateSignatureModeInfo(info *txtypes.ModeInfo, signature []byte) error {
	if info == nil {
		return fmt.Errorf("missing signature mode info")
	}
	switch mode := info.Sum.(type) {
	case *txtypes.ModeInfo_Single_:
		if mode.Single == nil || len(signature) == 0 {
			return fmt.Errorf("missing single signature data")
		}
	case *txtypes.ModeInfo_Multi_:
		if mode.Multi == nil || mode.Multi.Bitarray == nil || len(mode.Multi.Bitarray.Elems) == 0 || mode.Multi.Bitarray.ExtraBitsStored > 7 {
			return fmt.Errorf("invalid multisig bit array")
		}
		var raw cryptotypes.MultiSignature
		if err := raw.Unmarshal(signature); err != nil {
			return fmt.Errorf("invalid multisig encoding: %w", err)
		}
		if len(raw.Signatures) != len(mode.Multi.ModeInfos) || mode.Multi.Bitarray.NumTrueBitsBefore(mode.Multi.Bitarray.Count()) != len(raw.Signatures) {
			return fmt.Errorf("multisig signature count does not match mode infos and bit array")
		}
		for i, nested := range mode.Multi.ModeInfos {
			if err := validateSignatureModeInfo(nested, raw.Signatures[i]); err != nil {
				return fmt.Errorf("multisig member %d: %w", i, err)
			}
		}
	default:
		return fmt.Errorf("unknown signature mode info %T", info.Sum)
	}
	return nil
}

func readTxAndInitContexts(clientCtx client.Context, cmd *cobra.Command, filename string) (client.Context, tx.Factory, sdk.Tx, error) {
	stdTx, err := authclient.ReadTxFromFile(clientCtx, filename)
	if err != nil {
		return clientCtx, tx.Factory{}, nil, err
	}

	txFactory, err := tx.NewFactoryCLI(clientCtx, cmd.Flags())
	if err != nil {
		return clientCtx, tx.Factory{}, nil, err
	}

	return clientCtx, txFactory, stdTx, nil
}
