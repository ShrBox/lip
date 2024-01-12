package tooth

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/lippkg/lip/internal/context"
	"github.com/lippkg/lip/internal/network"

	"golang.org/x/mod/module"
)

// IsInstalled checks if a tooth is installed.
func IsInstalled(ctx context.Context, toothRepoPath string) (bool, error) {

	metadataList, err := GetAllMetadata(ctx)
	if err != nil {
		return false, fmt.Errorf(
			"failed to list all installed tooth metadata: %w", err)
	}

	for _, metadata := range metadataList {
		if metadata.ToothRepoPath() == toothRepoPath {
			return true, nil
		}
	}

	return false, nil
}

// GetAllMetadata lists all installed tooth metadata.
func GetAllMetadata(ctx context.Context) ([]Metadata, error) {
	metadataList := make([]Metadata, 0)

	metadataDir, err := ctx.MetadataDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get metadata directory: %w", err)
	}

	filePaths, err := filepath.Glob(filepath.Join(metadataDir.LocalString(), "*.json"))
	if err != nil {
		return nil, fmt.Errorf("failed to list metadata files: %w", err)
	}

	for _, filePath := range filePaths {
		jsonBytes, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read metadata file: %w", err)
		}

		metadata, err := MakeMetadata(jsonBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse metadata file: %w", err)
		}

		metadataList = append(metadataList, metadata)
	}

	// Sort the metadata list in case-insensitive ascending order of the tooth
	// repository.
	sort.Slice(metadataList, func(i, j int) bool {
		return strings.ToLower(metadataList[i].ToothRepoPath()) < strings.ToLower(
			metadataList[j].ToothRepoPath())
	})

	return metadataList, nil
}

// GetMetadata finds the installed tooth metadata.
func GetMetadata(ctx context.Context, toothRepoPath string) (Metadata,
	error) {

	metadataList, err := GetAllMetadata(ctx)
	if err != nil {
		return Metadata{}, fmt.Errorf(
			"failed to list all installed tooth metadata: %w", err)
	}

	for _, metadata := range metadataList {
		if metadata.ToothRepoPath() == toothRepoPath {
			return metadata, nil
		}
	}

	return Metadata{}, fmt.Errorf("cannot find installed tooth metadata: %v",
		toothRepoPath)
}

// GetAvailableVersions fetches the version list of a tooth repository.
// The version list is sorted in descending order.
func GetAvailableVersions(ctx context.Context, toothRepoPath string) (semver.Versions,
	error) {

	if err := module.CheckPath(toothRepoPath); err != nil {
		return nil, fmt.Errorf("invalid repository path: %v", toothRepoPath)
	}

	goModuleProxyURL, err := ctx.GoModuleProxyURL()
	if err != nil {
		return nil, fmt.Errorf("failed to get go module proxy URL: %w", err)
	}

	versionURL, err := network.GenerateGoModuleVersionListURL(toothRepoPath, goModuleProxyURL)
	if err != nil {
		return nil, fmt.Errorf("failed to generate version list URL: %w", err)
	}

	content, err := network.GetContent(versionURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch version list: %w", err)
	}

	reader := bytes.NewReader(content)

	// Each line is a version.
	versionList := make(semver.Versions, 0)
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		versionString := scanner.Text()
		versionString = strings.TrimPrefix(versionString, "v")
		versionString = strings.TrimSuffix(versionString, "+incompatible")
		version, err := semver.Parse(versionString)
		if err != nil {
			continue
		}
		versionList = append(versionList, version)
	}

	semver.Sort(versionList)

	// Reverse the version list.
	for i, j := 0, len(versionList)-1; i < j; i, j = i+1, j-1 {
		versionList[i], versionList[j] = versionList[j], versionList[i]
	}

	return versionList, nil
}

// GetLatestStableVersion returns the correct version of the tooth
// specified by the specifier.
func GetLatestStableVersion(ctx context.Context,
	toothRepoPath string) (semver.Version, error) {

	versionList, err := GetAvailableVersions(ctx, toothRepoPath)
	if err != nil {
		return semver.Version{}, fmt.Errorf(
			"failed to get available version list: %w", err)
	}

	for _, version := range versionList {
		if len(version.Pre) == 0 {
			return version, nil
		}
	}

	return semver.Version{}, fmt.Errorf("cannot find latest stable version")
}
