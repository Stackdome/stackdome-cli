package installer

import (
	"os"
	"strings"
	"testing"
)

func TestInstallersMatchReleaseArchiveContract(t *testing.T) {
	workflow := readContractFile(t, ".github/workflows/release.yml")
	unixInstaller := readContractFile(t, "install.sh")
	windowsInstaller := readContractFile(t, "install.ps1")

	requireContractText(t, "release workflow", workflow,
		`stackdome_${version}_${GOOS}_${GOARCH}.tar.gz`,
		`stackdome_${version}_${GOOS}_${GOARCH}.zip`,
		`sha256sum *.tar.gz *.zip > checksums.txt`,
	)
	requireContractText(t, "Unix installer", unixInstaller,
		`asset="stackdome_${version}_${os}_${arch}.tar.gz"`,
		`checksums_url="${release_base_url}/${version}/checksums.txt"`,
	)
	requireContractText(t, "Windows installer", windowsInstaller,
		`stackdome_{0}_windows_{1}.zip`,
		`$checksumsUrl = '{0}/{1}/checksums.txt'`,
	)
}

func TestReleaseWorkflowCanRepairExistingRelease(t *testing.T) {
	workflow := readContractFile(t, ".github/workflows/release.yml")
	requireContractText(t, "release workflow", workflow,
		"workflow_dispatch:",
		`gh release view "${RELEASE_TAG}"`,
		`gh release upload "${RELEASE_TAG}" dist/*.tar.gz dist/*.zip --clobber`,
		`gh release upload "${RELEASE_TAG}" dist/checksums.txt --clobber`,
		`gh release create "${RELEASE_TAG}"`,
	)
}

func TestReleaseWorkflowUsesInstallerSafeQualifiedTagRefs(t *testing.T) {
	workflow := readContractFile(t, ".github/workflows/release.yml")
	requireContractText(t, "release workflow", workflow,
		`[[ ! "${RELEASE_TAG}" =~ ^v[A-Za-z0-9._-]+$ ]]`,
		`printf 'tag_ref=refs/tags/%s\n' "${RELEASE_TAG}"`,
	)

	const qualifiedCheckout = `ref: ${{ needs.prepare.outputs.tag_ref }}`
	if count := strings.Count(workflow, qualifiedCheckout); count != 3 {
		t.Errorf("release workflow has %d qualified tag checkouts, want 3", count)
	}
}

func TestReleaseWorkflowSerializesRepairsAndPublishesChecksumsLast(t *testing.T) {
	workflow := readContractFile(t, ".github/workflows/release.yml")
	requireContractText(t, "release workflow", workflow,
		"concurrency:",
		`group: release-${{ github.event_name == 'workflow_dispatch' && inputs.tag || github.ref_name }}`,
		"cancel-in-progress: false",
		`gh release upload "${RELEASE_TAG}" dist/*.tar.gz dist/*.zip --clobber`,
		`gh release upload "${RELEASE_TAG}" dist/checksums.txt --clobber`,
	)

	archives := strings.Index(workflow, `gh release upload "${RELEASE_TAG}" dist/*.tar.gz dist/*.zip --clobber`)
	checksums := strings.Index(workflow, `gh release upload "${RELEASE_TAG}" dist/checksums.txt --clobber`)
	if archives >= checksums {
		t.Error("release repair must finish uploading archives before publishing checksums.txt")
	}
}

func TestPullRequestsRunInstallerAndReleaseContractTests(t *testing.T) {
	workflow := readContractFile(t, ".github/workflows/ci.yml")
	requireContractText(t, "CI workflow", workflow,
		"pull_request:",
		"go test ./...",
		"go vet ./...",
		"sh -n install.sh",
	)
}

func TestNewReleaseIsPublishedOnlyAfterAllAssetsExist(t *testing.T) {
	workflow := readContractFile(t, ".github/workflows/release.yml")
	requireContractText(t, "release workflow", workflow,
		`gh release create "${RELEASE_TAG}" --draft --verify-tag --generate-notes --title "Stackdome CLI ${RELEASE_TAG}"`,
		`gh release upload "${RELEASE_TAG}" dist/*.tar.gz dist/*.zip`,
		`gh release upload "${RELEASE_TAG}" dist/checksums.txt`,
		`gh release edit "${RELEASE_TAG}" --draft=false`,
	)

	createDraft := strings.Index(workflow, `gh release create "${RELEASE_TAG}" --draft`)
	uploadArchives := strings.LastIndex(workflow, `gh release upload "${RELEASE_TAG}" dist/*.tar.gz dist/*.zip`)
	uploadChecksums := strings.LastIndex(workflow, `gh release upload "${RELEASE_TAG}" dist/checksums.txt`)
	publish := strings.Index(workflow, `gh release edit "${RELEASE_TAG}" --draft=false`)
	if !(createDraft < uploadArchives && uploadArchives < uploadChecksums && uploadChecksums < publish) {
		t.Error("new release must remain a draft until archives and checksums are uploaded")
	}
}

func TestRepairPublishesAPreviouslyFailedDraftRelease(t *testing.T) {
	workflow := readContractFile(t, ".github/workflows/release.yml")
	requireContractText(t, "release workflow", workflow,
		`gh release view "${RELEASE_TAG}" --json isDraft --jq .isDraft`,
		`release_is_draft=true`,
		`if [[ "${release_is_draft}" == "true" ]]; then`,
		`gh release edit "${RELEASE_TAG}" --draft=false`,
	)

	uploadChecksums := strings.LastIndex(workflow, `gh release upload "${RELEASE_TAG}" dist/checksums.txt`)
	publishDraft := strings.LastIndex(workflow, `gh release edit "${RELEASE_TAG}" --draft=false`)
	if uploadChecksums < 0 || publishDraft < uploadChecksums {
		t.Error("draft release repair must publish only after checksums are uploaded")
	}
}

func readContractFile(t *testing.T, path string) string {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(contents)
}

func requireContractText(t *testing.T, sourceName, source string, values ...string) {
	t.Helper()
	for _, value := range values {
		if !strings.Contains(source, value) {
			t.Errorf("%s is missing release contract %q", sourceName, value)
		}
	}
}
