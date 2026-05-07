# Security policy

## Reporting a vulnerability

If you discover a security issue in `armup`, please report it privately by
opening a [security advisory](https://github.com/mihnen/armup/security/advisories/new).

Don't open a public issue or PR for the bug itself — that surfaces the
vulnerability before there's a fix available. Public issues are fine for
hardening suggestions or general questions.

I'll acknowledge reports within a few days and work with you on a fix and
disclosure timeline.

## Supported versions

Only the latest release on the [Releases page](../../releases/latest) is
supported. `armup` ships single-binary releases — please update before
reporting.

## Scope

In scope:

- Code execution or privilege escalation via `armup` itself or its install
  scripts (`install.sh`, `install.ps1`).
- Tampering with downloaded toolchain archives despite SHA-256 verification.
- Path-traversal / archive-slip in extraction.
- Injection vulnerabilities in any subcommand input.
- Anything that would let an attacker compromise a user's machine via a
  malicious mirror, MITM, or crafted ARM-published archive that we accept.

Out of scope:

- Vulnerabilities in the ARM toolchain itself — report those to ARM.
- Vulnerabilities in upstream Go modules — report upstream first;
  `armup` will pick up patched versions in a follow-up release.
- Issues that require an attacker to already have administrative access on
  the user's machine.
