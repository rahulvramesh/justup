package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/google/uuid"
	"github.com/rahulvramesh/justup/pkg/database"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"
)

var sshKeyCmd = &cobra.Command{
	Use:   "ssh-key",
	Short: "Manage SSH keys",
	Long: `Manage SSH public keys for workspace authentication.

Add your SSH public key to authenticate when connecting to workspaces
via the SSH proxy.`,
}

var sshKeyAddCmd = &cobra.Command{
	Use:   "add <path-to-public-key>",
	Short: "Add an SSH public key",
	Long: `Add an SSH public key for authentication.

Examples:
  justup ssh-key add ~/.ssh/id_ed25519.pub
  justup ssh-key add ~/.ssh/id_rsa.pub --name "work laptop"`,
	Args: cobra.ExactArgs(1),
	Run:  runSSHKeyAdd,
}

var sshKeyListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List registered SSH keys",
	Run:     runSSHKeyList,
}

var sshKeyRemoveCmd = &cobra.Command{
	Use:     "remove <fingerprint>",
	Aliases: []string{"rm", "delete"},
	Short:   "Remove an SSH key",
	Long: `Remove an SSH key by its fingerprint.

Use 'justup ssh-key list' to see key fingerprints.

Examples:
  justup ssh-key remove SHA256:abc123...`,
	Args: cobra.ExactArgs(1),
	Run:  runSSHKeyRemove,
}

var sshKeyName string

func init() {
	sshKeyCmd.AddCommand(sshKeyAddCmd)
	sshKeyCmd.AddCommand(sshKeyListCmd)
	sshKeyCmd.AddCommand(sshKeyRemoveCmd)

	sshKeyAddCmd.Flags().StringVarP(&sshKeyName, "name", "n", "", "Name for the key (defaults to filename)")

	// Add ssh-key command to root
	rootCmd.AddCommand(sshKeyCmd)
}

func getDBPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".justup", "justup.db")
}

func runSSHKeyAdd(cmd *cobra.Command, args []string) {
	keyPath := args[0]

	// Expand ~ in path
	if strings.HasPrefix(keyPath, "~/") {
		home, _ := os.UserHomeDir()
		keyPath = filepath.Join(home, keyPath[2:])
	}

	// Read the public key file
	keyBytes, err := os.ReadFile(keyPath)
	if err != nil {
		exitError("failed to read public key file", err)
	}

	// Parse the public key
	pubKey, comment, _, _, err := ssh.ParseAuthorizedKey(keyBytes)
	if err != nil {
		exitError("failed to parse public key", err)
	}

	// Get fingerprint
	fingerprint := ssh.FingerprintSHA256(pubKey)

	// Determine key name
	name := sshKeyName
	if name == "" {
		if comment != "" {
			name = comment
		} else {
			name = filepath.Base(keyPath)
		}
	}

	// Open database
	db, err := database.Open(getDBPath())
	if err != nil {
		exitError("failed to open database", err)
	}
	defer db.Close()

	// Get or create default user
	user, err := db.GetOrCreateDefaultUser()
	if err != nil {
		exitError("failed to get user", err)
	}

	// Check if key already exists
	existingKey, err := db.GetSSHKeyByFingerprint(fingerprint)
	if err == nil && existingKey != nil {
		fmt.Printf("Key already registered as '%s'\n", existingKey.Name)
		fmt.Printf("Fingerprint: %s\n", fingerprint)
		return
	}

	// Add the key
	sshKey := &database.SSHKey{
		ID:          uuid.New().String(),
		UserID:      user.ID,
		Name:        name,
		PublicKey:   strings.TrimSpace(string(keyBytes)),
		Fingerprint: fingerprint,
		CreatedAt:   time.Now(),
	}

	if err := db.AddSSHKey(sshKey); err != nil {
		exitError("failed to add SSH key", err)
	}

	fmt.Printf("SSH key added successfully!\n")
	fmt.Printf("  Name:        %s\n", name)
	fmt.Printf("  Fingerprint: %s\n", fingerprint)
	fmt.Printf("\nYou can now connect to workspaces via:\n")
	fmt.Printf("  ssh <workspace>@<proxy-host>\n")
}

func runSSHKeyList(cmd *cobra.Command, args []string) {
	db, err := database.Open(getDBPath())
	if err != nil {
		exitError("failed to open database", err)
	}
	defer db.Close()

	user, err := db.GetOrCreateDefaultUser()
	if err != nil {
		exitError("failed to get user", err)
	}

	keys, err := db.ListSSHKeys(user.ID)
	if err != nil {
		exitError("failed to list SSH keys", err)
	}

	if len(keys) == 0 {
		fmt.Println("No SSH keys registered.")
		fmt.Println("\nAdd one with:")
		fmt.Println("  justup ssh-key add ~/.ssh/id_ed25519.pub")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tFINGERPRINT\tADDED\tLAST USED")
	for _, key := range keys {
		lastUsed := "never"
		if key.LastUsedAt != nil {
			lastUsed = formatTimeAgo(*key.LastUsedAt)
		}
		added := formatTimeAgo(key.CreatedAt)
		// Truncate fingerprint for display
		fp := key.Fingerprint
		if len(fp) > 20 {
			fp = fp[:20] + "..."
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", key.Name, fp, added, lastUsed)
	}
	w.Flush()
}

func runSSHKeyRemove(cmd *cobra.Command, args []string) {
	fingerprint := args[0]

	// Confirm removal
	fmt.Printf("Remove SSH key with fingerprint '%s'? [y/N]: ", fingerprint)
	reader := bufio.NewReader(os.Stdin)
	response, _ := reader.ReadString('\n')
	response = strings.TrimSpace(strings.ToLower(response))

	if response != "y" && response != "yes" {
		fmt.Println("Cancelled.")
		return
	}

	db, err := database.Open(getDBPath())
	if err != nil {
		exitError("failed to open database", err)
	}
	defer db.Close()

	// Try to find key by partial fingerprint match
	user, err := db.GetOrCreateDefaultUser()
	if err != nil {
		exitError("failed to get user", err)
	}

	keys, err := db.ListSSHKeys(user.ID)
	if err != nil {
		exitError("failed to list SSH keys", err)
	}

	var matchedKey *database.SSHKey
	for _, key := range keys {
		if strings.Contains(key.Fingerprint, fingerprint) || key.Fingerprint == fingerprint {
			matchedKey = &key
			break
		}
	}

	if matchedKey == nil {
		exitError("SSH key not found", nil)
	}

	if err := db.DeleteSSHKey(matchedKey.ID); err != nil {
		exitError("failed to remove SSH key", err)
	}

	fmt.Printf("SSH key '%s' removed.\n", matchedKey.Name)
}

func formatTimeAgo(t time.Time) string {
	d := time.Since(t)

	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	if d < 30*24*time.Hour {
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
	return t.Format("2006-01-02")
}
