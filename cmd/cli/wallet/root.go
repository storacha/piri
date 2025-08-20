package wallet

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	leveldb "github.com/ipfs/go-ds-leveldb"
	logging "github.com/ipfs/go-log/v2"
	"github.com/spf13/cobra"

	"github.com/storacha/piri/pkg/config"
	"github.com/storacha/piri/pkg/store/keystore"
	"github.com/storacha/piri/pkg/wallet"
)

var log = logging.Logger("cli/wallet")

// TODO this needs to become a client command, rather than mutating the
// local filesystem directly.
var (
	Cmd = &cobra.Command{
		Use:   "wallet",
		Short: "Manage wallet addresses.",
	}
	ListCmd = &cobra.Command{
		Use:   "list",
		Short: "List all wallet addresses.",
		Args:  cobra.NoArgs,
		RunE:  doList,
	}
	ImportCmd = &cobra.Command{
		Use:   "import",
		Args:  cobra.ExactArgs(1),
		Short: "Import an address to the wallet.",
		Long: `Piri expects a delegated filecoin address private key in hex format as a single argument to this command.
You can create the required address via lotus using the command:
	'lotus wallet new delegated'
You can export the wallet from lotus using the command:
	'lotus wallet export <FILECOIN_ADDRESS>' > wallet.hex
You may then import 'wallet.hex' via this command`,
		Example: "piri wallet import wallet.hex",
		RunE:    doImport,
	}
)

func init() {
	Cmd.AddCommand(ListCmd)
	Cmd.AddCommand(ImportCmd)
}

func doList(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	cfg, err := config.Load[config.LocalConfig]()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// NB: sure we could just create one with mkdirp, but this allows us to inform
	// a user that they didn't have dir and where we are creating one now
	if _, err := os.Stat(cfg.Repo.DataDir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.Infof("data dir not found, creating one at %s", cfg.Repo.DataDir)
			if err := os.MkdirAll(cfg.Repo.DataDir, 0700); err != nil {
				return fmt.Errorf("creating data dir: %w", err)
			}
		}
	}

	walletDir := filepath.Join(cfg.Repo.DataDir, "wallet")
	if err := os.MkdirAll(walletDir, 0755); err != nil {
		return fmt.Errorf("creating wallet data dir at %s: %w", walletDir, err)
	}

	pdpDs, err := leveldb.NewDatastore(walletDir, &leveldb.Options{
		ReadOnly: true,
	})
	if err != nil {
		return err
	}
	defer pdpDs.Close()

	keyStore, err := keystore.NewKeyStore(pdpDs)
	if err != nil {
		return err
	}

	wlt, err := wallet.NewWallet(keyStore)
	if err != nil {
		return err
	}

	kis, err := wlt.List(ctx)
	if err != nil {
		return err
	}

	for _, k := range kis {
		cmd.Println("Address: ", k.Address.String())
	}

	return nil
}

func doImport(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	cfg, err := config.Load[config.LocalConfig]()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// NB: sure we could just create one with mkdirp, but this allows us to inform
	// a user that they didn't have dir and where we are creating one now
	if _, err := os.Stat(cfg.Repo.DataDir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.Infof("data dir not found, creating one at %s", cfg.Repo.DataDir)
			if err := os.MkdirAll(cfg.Repo.DataDir, 0700); err != nil {
				return fmt.Errorf("creating data dir: %w", err)
			}
		}
	}

	inpdata, err := os.ReadFile(args[0])
	if err != nil {
		return err
	}

	data, err := hex.DecodeString(strings.TrimSpace(string(inpdata)))
	if err != nil {
		return err
	}

	var ki struct {
		Type       string
		PrivateKey []byte
	}
	if err := json.Unmarshal(data, &ki); err != nil {
		return err
	}

	walletDir := filepath.Join(cfg.Repo.DataDir, "wallet")
	if err := os.MkdirAll(walletDir, 0755); err != nil {
		return fmt.Errorf("creating wallet data dir at %s: %w", walletDir, err)
	}

	pdpDs, err := leveldb.NewDatastore(walletDir, nil)
	if err != nil {
		return err
	}

	keyStore, err := keystore.NewKeyStore(pdpDs)
	if err != nil {
		return err
	}

	wlt, err := wallet.NewWallet(keyStore)
	if err != nil {
		return err
	}

	addr, err := wlt.Import(ctx, &keystore.KeyInfo{PrivateKey: ki.PrivateKey})
	if err != nil {
		return err
	}

	fmt.Printf("imported wallet %s successfully!\n", addr)
	return nil
}
