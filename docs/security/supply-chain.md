# Supply chain of the icon packs

Case 7 of the [plan](../planning/project-plan.md) §12: five catalogues of third-party files go into
the backend image at every build, and their maintainers and hosting decide their content.

| Catalogue | Pinned in | By | Verified at build |
|---|---|---|---|
| `mdi`, `material-symbols`, `si` | [backend/icons/package-lock.json](../../backend/icons/package-lock.json) | Version, the tarball's URL on `registry.npmjs.org` and its sha512 integrity | The tarball against its integrity, before it is read |
| `sh` (selfh.st), `di` (Dashboard Icons) | [backend/icons/sources.yaml](../../backend/icons/sources.yaml) | A full commit hash and the SHA-256 of the files used — `svg/` and the index | The checkout against the SHA-256, before it is read |

- **Every change is a pull request** through every tier: Renovate moves the npm pins (minor and patch
  automerged after CI) and the commits (weekly, by hand: the pull request fails the image build until
  the new SHA-256 is put beside the commit after `make icon-pins`, which makes a person look).
- **Every SVG passes the content check** at build ([served-icons.md](served-icons.md)); a file that
  fails is left out and counted in the pack's manifest.
- **`required.txt`** holds what must keep resolving — the owner's icon names among them — so an
  update cannot silently drop an icon in use.
- The image passes Trivy and ClamAV like every other build (`release.yml`).
- `git`, which fetches the two commits, exists in the build stage only; the image is distroless.

## What this does not cover

### H-10 The content of a pinned change is not reviewed

The pins prove that what is built is what was pinned; they do not review what a new commit of a logo
catalogue changed among thousands of files. The SHA-256 step makes a person look at the pull
request, not at every file.
