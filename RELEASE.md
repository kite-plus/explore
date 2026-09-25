# Releasing

Cutting a release is: write both changelogs, point the deploy steps at the version, tag and push. The release workflow does the rest.

## 1. Write the changelogs

In `CHANGELOG.md` and `CHANGELOG.zh-CN.md`, add a `## [<version>] - <date>` section below the `Unreleased` (`未发布`) heading, which stays and is left empty, and move that heading's items into it.

Start the section with a sentence or two on what the release is about, then group the changes by feature under Added, Changed and Fixed (新增、变更、修复). Write for the people who read or run the site, not in commit subjects: what changed, and what it means for them.

The release notes are built from these sections by `scripts/release-notes.mjs`. The Chinese section is the body of the GitHub Release, the English one is linked from it, and a download section and a compare link follow. To see what the release will carry:

```bash
PREVIOUS_TAG=v0.1.4 node scripts/release-notes.mjs v0.1.5
```

A version without a section in both files fails the release before any image is pushed.

## 2. Point the deploy steps at the version

`README.md` and `README.zh-CN.md` name the version twice each: in the `base=` URL the deployment files come from, and in `EXPLORE_VERSION=`. Commit them as `docs: point the deploy steps at v<version>`.

## 3. Check

```bash
make check
make web-test
```

`make check` runs the store and API tests against the database in `EXPLORE_TEST_DATABASE_URL`.

## 4. Tag and push

```bash
git tag -a v<version> -m v<version>
git push origin main v<version>
```

Pushing the tag starts `.github/workflows/release.yml`. It runs CI's checks, builds the notes, and then publishes:

- the images `ghcr.io/kite-plus/explore:<version>` and `ghcr.io/kite-plus/explore-web:<version>` for amd64 and arm64, also tagged `<major>.<minor>` and `latest`;
- the native archives `explore-<version>-<os>-<arch>` for Linux, macOS and Windows on amd64 and arm64, `explore-web-<version>.tar.gz` and `SHA256SUMS.txt`;
- the GitHub Release `Explore v<version>` with the notes and the archives.

A tag with a `-` suffix, such as `v0.2.0-rc.1`, is a prerelease: it does not move `latest`, and its GitHub Release is marked as a prerelease. Re-running the workflow for a tag updates its release instead of failing. A manual run builds the images and archives without publishing anything.

## 5. Upgrade servers

As the README says: set `EXPLORE_VERSION` to the new version, then `docker compose pull && docker compose up -d`. The database is migrated before the new version starts. Without Docker, run `explore migrate up` with the new binary before starting it.
