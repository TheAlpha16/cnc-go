# cnc-go
A lightweight Go package for distributed command execution and coordination across services

---

## What it does

`cnc-go` lets you:

- ⚡ Register commands with custom handlers

- 📡 Trigger commands locally **or across a cluster**

- 🔌 Propagate events using pluggable Pub/Sub backends (Redis/Valkey included, extendable to NATS, Kafka, etc.)

- 🗄️ Ensure synchronized actions across pods/nodes

- 🛠️ Gracefully start, run, and shutdown CNC workers

---


## 🤔 Why?

In distributed systems, some actions — like invalidating caches, reloading configs, or resetting feature flags — must be executed **everywhere**.

Instead of building ad-hoc RPCs or reinventing messaging, **cnc-go provides a simple abstraction**:
publish a command once and have every node execute it reliably.

---

## 🚀 Features

- 🔌 **Pluggable Pub/Sub transport** (Redis included, extendable to NATS/Kafka/etc.)

- 🧩 **Simple API** for registering and triggering commands

- 🛡️ **Typed errors** for safe error handling (`ErrHandlerNotFound`, `ErrHandlerAlreadyExists`, …)

- 📦 **Clean, idiomatic Go design** (no external dependencies beyond the transport driver)

- ✅ Unit tested & extensible

---

## 📦 Installation

```bash
go get github.com/TheAlpha16/cnc-go
```
