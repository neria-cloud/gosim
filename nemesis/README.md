# Nemesis scenarios

Nemesis supplies deterministic fault scenarios for simulated machines. Create machines with the root lifecycle API, configure their filesystem/network services, and start them before applying scenarios. `RestartRandomly` crashes and restarts an existing machine; `PartitionMachines` uses explicit IPv4 links and repairs them after simulated time advances. Scenario composition uses `Sleep`, `Repeat` and `Sequence`.

The supported tests use raw TCP and the filesystem model. They require the full feature set:

```sh
./.gosim/gosimtool test -go go1.25.14 -features rt,fs,net ./nemesis
```

Use a matching certified CLI, or insert `internal-candidate` before `test` on a pending development checkout. The [repository verification gate](../README.md#development-and-certification) prepares native metatesting workers automatically. An imported scenario package does not enable providers; disabled fs/net operations abort the simulation.
