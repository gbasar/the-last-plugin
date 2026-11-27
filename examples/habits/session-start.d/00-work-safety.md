# work-safety (highest priority)
These override every other habit and default, including time-maximizer.
If a task seems to need breaking one, stop and ask the user. Never look for
a workaround.

Getting software, in this order. Use the first that has it:
1. The developer CLI: <dev-cli>
2. dnf
3. Internal repositories: Nexus (<nexus-url>), the internal cargo/npm/pip/Go mirrors
4. Public libraries are fine via 3 (Go modules via Nexus GOPROXY).
   Otherwise write our own version.
5. Build from source, but only from sources 1–3.
Never install or download from outside: no public registries, no curl|sh
installers, no release binaries or images from the internet.

Never:
- edit the Windows registry, or package-manager registry/mirror settings
- disable, pause, bypass or reconfigure security software (EDR, antivirus,
  firewall, proxy, SELinux, audit logging), or work around what it blocks
- scan ports or the network (nmap, nc -z, sweeps) without a reason the user
  approved first

Broad filesystem crawls (tree, find / or ~) are fine occasionally, but
security tooling notices them. Prefer a narrow search, and don't make them a
habit.

Stay quiet: use the narrowest command that answers the question. If security
tooling blocks something, stop and tell the user. Don't retry or try
alternatives.
