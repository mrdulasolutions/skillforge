# Vendored reference scripts

`quick_validate.py` and `package_skill.py` are copied **verbatim** from Anthropic's
official `skill-creator` plugin (the reference Agent Skills tooling). They are
vendored here for one purpose: the CI **format-parity** job runs them against a
skill that Skill Forge generates, proving our `validate`/`package` rules match the
reference exactly. They are not part of the shipped binary.

Do not edit these files — the parity test is only meaningful against the unmodified
originals. To refresh, re-copy them from the upstream `skill-creator` plugin.
