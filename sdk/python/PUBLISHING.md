# Publishing `pamone-obs` to PyPI

The name `pamone-obs` is available on PyPI. We publish via **Trusted Publishing**
(GitHub OIDC) — no API token stored anywhere.

## 1) Register a "pending publisher" on PyPI
PyPI → Account → **Publishing** → *Add a new pending publisher*, fill exactly:

| Field | Value |
|-------|-------|
| PyPI Project Name | `pamone-obs` |
| Owner | `bennnr` |
| Repository name | `pamone-observability` |
| Workflow name | `publish.yml` |
| Environment name | `pypi` *(optional; if set it must match the job's `environment:`)* |

The "pending" publisher lets this workflow create the project on first publish;
afterwards it becomes a normal trusted publisher.

## 2) GitHub environment (optional but recommended)
Repo → Settings → Environments → create **`pypi`** (matches the workflow). You
can add protection rules (required reviewers) if you want manual approval.

## 3) Bump the version
Edit `sdk/python/pyproject.toml` → `version`. PyPI rejects re-uploading an
existing version, so every release needs a new one (e.g. `0.1.1`).

## 4) Trigger the publish
Create a GitHub Release (**Releases → Draft a new release**) with a tag such as
`v0.1.0`, then **Publish release**. The workflow
`.github/workflows/publish.yml` builds `sdist` + `wheel` from `sdk/python/` and
publishes over OIDC. You can also run it manually via **Actions → publish →
Run workflow**.

## 5) Verify
```bash
pip index versions pamone-obs
pip install pamone-obs
python -c "import pamone_obs, pamone_obs.metrics, pamone_obs.trace; print(pamone_obs.__version__)"
```

## Alternative: API token + twine (no OIDC)
If you prefer manual uploads:
```bash
cd sdk/python
python -m build
pip install twine
twine upload dist/*        # prompts for username `__token__` + an API token
```
(Trusted Publishing is preferred because no secret is stored.)

## Notes
- The workflow publishes only what `python -m build` produces; the package
  contains the `pamone_obs` module and the `pamone-obs` console script.
- To release from the whole-repo checkout, `packages-dir: sdk/python/dist` is set
  in the workflow (paths are relative to the repo root).
- `license = { text = "Proprietary" }` in `pyproject.toml` — change if a public
  license is intended before publishing.