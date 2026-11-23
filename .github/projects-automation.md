# HTTT Projects automation pilot

This pilot adds newly opened HTTT issues and same-repository pull requests to
[Herdworks project 4](https://github.com/orgs/herdworks/projects/4). It upgrades
`actions/add-to-project` from v1.0.2 to v2.0.1 and replaces the stored installation
token with a fresh GitHub App installation token for each eligible run. It is
not a rollout to other repositories and does not backfill existing items.

## Prerequisite: credential isolation decision

Before provisioning, an organization administrator must verify the existing
**Herdworks Action tokens** App's installations, repository selection, and
permissions. Its HTTT selection and organization Projects read/write were
reported as verified; repository Issues and Pull requests read permissions
remain unverified. No App access or credential has been changed by this patch.

If that App/key can reach unrelated private repositories, use a **dedicated
Projects-only pilot App/credential installed only on herdworks/httt** instead.
A signing key can mint tokens for the App's other installations; requesting a
restricted token in this workflow does not constrain a stolen signing key.
Do not copy or broaden private-repository CI-fetch secrets into public HTTT.
Do not reuse the expired `PROJECTS_AUTOMATION_TOKEN` as a signing key.

## Administrator provisioning checklist (required before live validation)

1. Select the isolated App approved above. Configure organization **Projects:
   read and write**, repository **Issues: read-only** and **Pull requests:
   read-only**; Metadata read is implicit. Repository Projects permission is
   not a substitute for organization Projects. No Contents, Actions, Workflows,
   or repository write permission is needed.
2. Install it on the `herdworks` organization with **Only select repositories →
   httt**. Confirm the installation has accepted the required permissions and
   can access organization project 4. Organization Projects permission is not
   a per-project-4 security boundary; consider this in the approval decision.
3. In `herdworks/httt` → Settings → Secrets and variables → Actions → Variables,
   create repository variable `HTTT_PROJECTS_APP_CLIENT_ID` with the selected
   App's nonsecret **Client ID** from its settings page. This is not an
   installation ID, App slug, or token.
4. Through the administrator's approved secure credential process, provision
   the isolated App's PEM private key as repository Actions secret
   `HTTT_PROJECTS_APP_PRIVATE_KEY` (literal multiline PEM, not base64). Never
   put it in source, logs, PR text, or chat. Key generation, secret creation,
   rotation, and access changes require separate authorization and are not
   performed by this pilot patch.

These names are purpose-specific configuration placeholders; this change
does not provision their values. Missing either value produces an explicit
configuration error and stops before minting or calling Projects, rather than
repeating the old `Bad credentials` error. Invalid nonempty credentials and
unaccepted installation permissions still require administrator diagnosis.

## Event and token boundaries

- `issues: opened` runs the default-branch workflow. Issue content is untrusted
  metadata and is never interpolated into shell commands.
- `pull_request_target: opened` uses the base-branch workflow, with a job guard
  requiring the head repository to be HTTT. Fork PRs and Dependabot PRs are
  excluded from this pilot, even though the target event can receive secrets.
  A run may be recorded with the job skipped. Same-repo PRs are metadata-only;
  this workflow never checks out code, fetches PR refs, uses local actions,
  runs builds, restores caches, or executes PR-controlled text.
- Keep `permissions: {}` for `GITHUB_TOKEN`. The App token is requested only for
  `herdworks/httt`, with organization Projects write, Issues read, and Pull
  requests read. No all-installation repository fallback is used. This limits
  repository access, while organization Projects access remains organization
  scoped. The actions are pinned to immutable full commits with version comments.
- `create-github-app-token` masks its output and revokes the token in its post
  step by default. Installation tokens expire after one hour if revocation
  cannot complete. The five-minute job does not pass its token to another job.
- Both pinned actions use Node 24 on GitHub-hosted `ubuntu-latest`.

## Validation and later reuse

Local checks must validate workflow syntax, pinned input names, the event guard,
and missing-configuration fail-fast behavior. Live App authentication and adding
an issue/PR to project 4 remain pending until authorized provisioning and review.
Do not treat local checks as proof of live access.

After approval and merge, separately authorize a live test using a newly opened
HTTT issue and same-repo PR. Confirm the run uses the approved App installation,
adds each item to project 4, and completes token cleanup without displaying
tokens or keys. Confirm fork PRs do not mint a token. Do not rerun the old failed
credentialed runs as a validation shortcut. Existing items need a separately
approved backfill plan.

For later reuse, repeat the App/key isolation review, repository selection,
project target, event policy, and permission checks per repository; do not copy
this secret or expand the installation automatically. Rollback is a reviewed
revert or workflow disablement, not restoring an expired installation token.

## Official references

- [GitHub Projects with App authentication and required permissions](https://docs.github.com/en/issues/planning-and-tracking-with-projects/automating-your-project/automating-projects-using-actions)
- [add-to-project v2.0.1 README and event semantics](https://github.com/actions/add-to-project/blob/f2c1f1fa649a891be84948ea89b5dfba1526ef05/README.md)
- [add-to-project v2.0.1 manifest](https://github.com/actions/add-to-project/blob/f2c1f1fa649a891be84948ea89b5dfba1526ef05/action.yml)
- [create-github-app-token v3.4.0 README](https://github.com/actions/create-github-app-token/blob/f418cff399f7aef4c2f0ad47e25ca7a6abbdd6fc/README.md)
- [create-github-app-token v3.4.0 manifest and permission inputs](https://github.com/actions/create-github-app-token/blob/f418cff399f7aef4c2f0ad47e25ca7a6abbdd6fc/action.yml)
- [GitHub event contexts and pull_request_target security](https://docs.github.com/en/actions/reference/workflows-and-actions/events-that-trigger-workflows#pull_request_target)
