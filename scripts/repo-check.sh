#!/usr/bin/env bash
# Repository policy: AGENTS.md is the only agent instruction file, and nothing that identifies a
# real account is committed (captured API answers, token caches, real e-mail addresses).
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

fail=0
for f in CLAUDE.md .cursorrules .windsurfrules .clinerules .github/copilot-instructions.md; do
  if git ls-files --error-unmatch "$f" >/dev/null 2>&1; then
    echo "repo-check: $f is not allowed; AGENTS.md is the only agent instruction file" >&2
    fail=1
  fi
done

# token caches and captured answers
if git ls-files | grep -E '(^|/)token\.json$|\.har$'; then
  echo "repo-check: a token cache or a captured HTTP session is committed" >&2
  fail=1
fi

# JSON Web Tokens (three base64url parts starting with {")
if git grep -nIE 'eyJ[A-Za-z0-9_-]{10,}\.eyJ[A-Za-z0-9_-]{10,}\.' -- . ':!scripts/repo-check.sh'; then
  echo "repo-check: a JSON Web Token is committed" >&2
  fail=1
fi

# e-mail addresses other than made-up ones and the GitHub noreply address
if git grep -nIhoE '[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}' -- . ':!scripts/repo-check.sh' ':!go.sum' ':!tools/' \
  | grep -vE '@example\.(com|org|net)$|@users\.noreply\.github\.com$' | grep .; then
  echo "repo-check: a real-looking e-mail address is committed (use example.com)" >&2
  fail=1
fi

exit $fail
