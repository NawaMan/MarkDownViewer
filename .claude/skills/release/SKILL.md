---
name: release
description: Cut and publish a viewmd release — bump version.txt, sync the README banner sample, commit, tag, push, and verify what GitHub published. Use when asked to release, cut a version, ship a version, or publish a new viewmd build.
---

# Releasing viewmd

`.github/workflows/release.yml` does the building and publishing. A release is
therefore three local commits' worth of work and one push: **bump, commit, tag,
push the tag**. Everything after that is watching and verifying.

Pushing the tag publishes a public GitHub release. If the user has not clearly
asked for that ("release this", "ship 0.7.0"), stop after the local tag and ask.

## 1. Preflight

The tree must be clean and on `main`, and the work being released must already
be committed — the release commit touches only `version.txt` and the README.

```bash
git status --short && git log --oneline -3
```

Run the suite. Per AGENTS.md this belongs in the booth:

```bash
./booth exec --run -- bash -lc 'cd /home/coder/code && gofmt -l cmd && go vet ./... && go test ./...'
```

If the booth cannot start (its binary download has 404'd for darwin-arm64
before), fall back to a host Go toolchain and say so in the report:

```bash
gofmt -l cmd && go vet ./... && go test ./...
```

`TestExposeDoesNotBlockServer` fails on a macOS host outside the booth — its
`booth--expose` stub expects the container. Confirm it also fails on the
previous release commit before dismissing it; CI on Linux is the authority.

Cross-compilation is worth a check when the change touched build-tagged files,
since a `_windows.go` typo only shows up here:

```bash
GOOS=windows go vet ./... && GOOS=linux GOARCH=arm64 go vet ./...
```

## 2. Pick the version

Pre-1.0, so: a new flag, a new capability, or a changed default → **minor**
(0.5.0 → 0.6.0). Fixes and docs only → **patch**. A changed default is a minor
bump even when the diff is small, because it changes what existing invocations
do.

## 3. Bump

`version.txt` is the single source of truth — `build.sh` stamps it into
`main.version` via ldflags, and the release workflow refuses a tag that
disagrees with it.

```bash
printf '0.6.0\n' > version.txt
```

The README's daemon sample prints the version, so it has to move too:

```bash
perl -pi -e 's/# viewmd v0\.5\.0 running in background/# viewmd v0.6.0 running in background/' README.md
grep -rn "0\.5\.0" --include='*' . | grep -v "^./.git/"   # must come back empty
```

If the release changed the banner, the flag list, or the usage text, make the
README's `## Usage` block and daemon sample match the real output before
tagging — run the binary and copy what it actually prints.

## 4. Commit and tag

AGENTS.md: no `Co-Authored-By`, no agent attribution of any kind. The subject
is exactly `Release X.Y.Z`, matching every release commit before it.

```bash
git add -A && git commit -m "Release 0.6.0"
git tag v0.6.0
```

## 5. Push

`main` first, then the tag — the tag push is what starts the release workflow.

```bash
git push origin main && git push origin v0.6.0
```

## 6. Watch

Two runs start: `CI` on the branch and `Release` on the tag.

```bash
gh run list --limit 4
gh run watch <release-run-id> --exit-status --interval 20
```

The Release run resolves the tag against `version.txt`, builds all six targets
once, *runs* each on a native runner of its own platform, checks the Linux
x86-64 binary on Alpine, then publishes and installs the published `install.sh`
to prove it works. Roughly two minutes.

## 7. Verify what shipped

```bash
gh release view v0.6.0 --json tagName,name,isDraft,isPrerelease,url,assets \
  --jq '{tag:.tagName,name:.name,draft:.isDraft,pre:.isPrerelease,assets:[.assets[].name]}'
curl -sI https://github.com/NawaMan/MarkDownViewer/releases/latest | grep -i "^location:"
```

Nine assets are expected: six binaries, `install.sh`, `install.ps1`,
`SHA256SUMS`. The redirect is how you confirm the Latest badge — `gh release
view` has no `isLatest` field, and `--json` errors out if you ask for one.

## Notes worth keeping

- The Latest badge goes to the highest version, not the newest push, so
  re-releasing an older tag cannot displace a newer one. A pre-release
  (`v0.7.0-rc1`) never takes it. There is no rolling `latest` tag — don't make
  one; `releases/latest/download/<asset>` resolves through the badge.
- A tag whose `version.txt` disagrees fails fast in "Resolve the tag" with a
  clear message. Fix `version.txt`, delete and re-push the tag.
- To re-publish an existing tag, dispatch the workflow with `force` enabled
  (`gh workflow run release.yml -f force=true`); it deletes the release, never
  the tag.
- Undoing a bad release: `gh release delete vX.Y.Z --yes` and
  `git push --delete origin vX.Y.Z`. Prefer shipping a patch — anyone who
  already downloaded the assets keeps them either way.
- The workflows warn about Node 20 deprecation and a missing `go.sum` for the
  Go cache. Both are expected for a zero-dependency module and neither fails
  the run.
