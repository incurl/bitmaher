package bindings

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// The peer blocklist.
//
// The engine refuses to talk to the peers on the list. The list is a
// file of addresses, and it is fetched from the address the user chose.
// The rules are the ones Rayburst uses, so that the two keep the same
// list.

// The names of the files and the folders of the blocklist.
const (
	// BlocklistFolder is the folder the blocklist lives in.
	BlocklistFolder = "bt"
	// BlocklistFile is the name of the blocklist.
	BlocklistFile = "peer-blocklist.txt"
)

// The bounds of the blocklist.
const (
	// MaxBlocklistSize is the largest a blocklist may be.
	MaxBlocklistSize = 16 * 1024 * 1024
	// BlocklistTimeout bounds the fetch of a blocklist.
	BlocklistTimeout = 30 * time.Second
)

// DefaultBlocklistURL is where the blocklist comes from.
//
// It is the list of every peer that is known to be harmful, gathered
// from the lists of the other projects.
const DefaultBlocklistURL = "https://bcr.pbh-btn.com/combine/all.txt"

// BtPeerBlocklistStatus is the state of the blocklist.
type BtPeerBlocklistStatus struct {
	// RuleCount is the number of rules in the list.
	RuleCount int `json:"ruleCount"`
	// FileSize is the size of the file, in bytes.
	FileSize int64 `json:"fileSize"`
	// Modified is when the file was last written, in milliseconds since
	// the epoch.
	Modified int64 `json:"modified"`
	// Source is where the list came from.
	Source string `json:"source"`
	// Bundled reports whether the list is the one that came with the
	// application, rather than one that was fetched.
	Bundled bool `json:"bundled"`
}

// GetBtPeerBlocklistStatus returns the state of the blocklist.
//
// The list is read from the file the application keeps, and it is not
// fetched: the status is about what is on the disk.
func (s *AppService) GetBtPeerBlocklistStatus() (BtPeerBlocklistStatus, error) {
	path, err := s.blocklistPath()
	if err != nil {
		return BtPeerBlocklistStatus{}, err
	}

	return readBlocklistStatus(path)
}

// SyncBtPeerBlocklist fetches the blocklist and gives it to the engine.
//
// The list that is on the disk is kept when the fetch fails: a list
// that is a day old is better than no list at all.
func (s *AppService) SyncBtPeerBlocklist() (BtPeerBlocklistStatus, error) {
	return s.reconcileBlocklist(true)
}

// ReconcileBtPeerBlocklist gives the engine the blocklist that is on
// the disk.
//
// It is what the application does when the engine starts: the engine
// has to know the list, and the file already holds it.
func (s *AppService) ReconcileBtPeerBlocklist() (BtPeerBlocklistStatus, error) {
	return s.reconcileBlocklist(false)
}

// reconcileBlocklist fetches the list, when asked to, and gives it to
// the engine.
func (s *AppService) reconcileBlocklist(fetch bool) (BtPeerBlocklistStatus, error) {
	path, err := s.blocklistPath()
	if err != nil {
		return BtPeerBlocklistStatus{}, err
	}

	if fetch {
		content, err := fetchBlocklist(DefaultBlocklistURL)
		if err != nil {
			// The list that is on the disk is kept.
			status, statusErr := readBlocklistStatus(path)
			if statusErr != nil {
				return BtPeerBlocklistStatus{}, err
			}
			return status, nil
		}

		normalised, count, err := normaliseBlocklist(content, DefaultBlocklistURL)
		if err != nil {
			return BtPeerBlocklistStatus{}, err
		}

		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return BtPeerBlocklistStatus{}, fmt.Errorf("the blocklist folder could not be created: %w", err)
		}
		if err := os.WriteFile(path, []byte(normalised), 0o600); err != nil {
			return BtPeerBlocklistStatus{}, fmt.Errorf("the blocklist could not be written: %w", err)
		}

		if s.console != nil {
			s.console.Info("aria2", fmt.Sprintf("the peer blocklist was updated: %d rules", count))
		}
	}

	// The engine is told the rules of the list.
	rules, err := readBlocklistRules(path)
	if err == nil {
		if client := s.engineClient(); client != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			if err := client.SetBtPeerBlocklist(ctx, rules); err != nil && s.console != nil {
				s.console.Warn("aria2", "the engine could not be given the peer blocklist: "+err.Error())
			}
		}
	}

	return readBlocklistStatus(path)
}

// blocklistPath returns the path of the blocklist.
func (s *AppService) blocklistPath() (string, error) {
	if s.dataDir == "" {
		return "", fmt.Errorf("the data directory is not known")
	}
	return filepath.Join(s.dataDir, BlocklistFolder, BlocklistFile), nil
}

// readBlocklistStatus returns the state of the blocklist on the disk.
func readBlocklistStatus(path string) (BtPeerBlocklistStatus, error) {
	status := BtPeerBlocklistStatus{}

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			// There is no list yet. That is not a failure: the
			// status is the one of a list that is not there.
			return status, nil
		}
		return status, fmt.Errorf("the blocklist could not be read: %w", err)
	}

	status.FileSize = info.Size()
	status.Modified = info.ModTime().UnixMilli()

	rules, err := readBlocklistRules(path)
	if err != nil {
		return status, err
	}
	status.RuleCount = len(rules)

	// The source is the first line of the file, when it is written
	// there.
	if content, err := os.ReadFile(path); err == nil {
		for _, line := range strings.Split(string(content), "\n") {
			if strings.HasPrefix(line, "# Source: ") {
				status.Source = strings.TrimPrefix(line, "# Source: ")
				break
			}
		}
	}

	return status, nil
}

// readBlocklistRules reads the rules of the blocklist.
//
// The rules are the addresses of the peers, and the ones that are not
// addresses are left out.
func readBlocklistRules(path string) ([]string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("the blocklist could not be read: %w", err)
	}

	rules := []string{}
	for _, line := range strings.Split(string(content), "\n") {
		value := strings.TrimSpace(line)
		if value == "" || strings.HasPrefix(value, "#") {
			continue
		}
		rules = append(rules, value)
	}
	return rules, nil
}

// normaliseBlocklist cleans a fetched blocklist.
//
// The comments are left out, the rules that are not addresses are
// refused, and the ones that are there twice are kept once.
func normaliseBlocklist(content, source string) (string, int, error) {
	rules := []string{}
	seen := map[string]bool{}

	for _, line := range strings.Split(content, "\n") {
		value := strings.TrimSpace(line)
		if value == "" || strings.HasPrefix(value, "#") {
			continue
		}

		// A rule that is not an address is a mistake, and it is
		// left out rather than given to the engine.
		if !isAddressRule(value) {
			continue
		}

		if seen[value] {
			continue
		}
		seen[value] = true
		rules = append(rules, value)
	}

	var builder strings.Builder
	builder.WriteString("# Source: " + source + "\n")
	for _, rule := range rules {
		builder.WriteString(rule + "\n")
	}

	return builder.String(), len(rules), nil
}

// isAddressRule answers whether a rule is an address or a range of
// addresses.
//
// The engine accepts an address on its own and a range in the form of
// a prefix, for both the fourth and the sixth version.
func isAddressRule(value string) bool {
	if strings.Contains(value, "/") {
		_, _, err := net.ParseCIDR(value)
		return err == nil
	}

	address := net.ParseIP(value)
	return address != nil
}

// fetchBlocklist downloads a blocklist.
//
// The answer is read a piece at a time, so that a list that is too big
// does not fill the memory.
func fetchBlocklist(url string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), BlocklistTimeout)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("the blocklist could not be asked for: %w", err)
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("the blocklist could not be fetched: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("the blocklist server answered HTTP %s", response.Status)
	}

	var builder strings.Builder
	read := int64(0)

	buffer := make([]byte, 32*1024)
	for {
		count, err := response.Body.Read(buffer)
		if count > 0 {
			read += int64(count)
			if read > MaxBlocklistSize {
				return "", fmt.Errorf("the blocklist is larger than %d bytes", MaxBlocklistSize)
			}
			builder.Write(buffer[:count])
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("the blocklist could not be read: %w", err)
		}
	}

	return builder.String(), nil
}
