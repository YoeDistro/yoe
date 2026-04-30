# Roadmap

## Developer Experience

- Feed server
  - `yoe` app creates a feed server.
  - Configure apk on device to use it.
- Software update
  - Yoe updater or SWUpdate
  - Rewrite in Zig?

## Needed Units

Units needed for a complete base Linux system. Existing units can be found via
`yoe list` or by browsing `modules/units-core/units/`.

### Medium Priority — Networking and Security

- [ ] `nftables` — modern firewall (preferred over legacy iptables). Requires
      new dep units `libmnl`, `libnftnl`, and `gmp` before it can be written.

### Low Priority — Nice to Have

- [ ] `dbus` — IPC message bus; dependency for many higher-level services. Pulls
      in expat (already present) plus a service supervisor — non-trivial, defer
      until a unit needs it.
