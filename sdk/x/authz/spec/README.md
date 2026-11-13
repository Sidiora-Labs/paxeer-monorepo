<!--
order: 0
title: Authz Overview
parent:
  title: "authz"
-->

# `authz`

## Abstract

`x/authz` is an implementation of a Cosmos SDK module, per upstream Cosmos SDK ADR 030, that allows
granting arbitrary privileges from one account (the granter) to another account (the grantee). Authorizations must be granted for a particular Msg service method one by one using an implementation of the `Authorization` interface.

## Contents

1. **[Concept](01_concepts.md)**
    - [Authorization](01_concepts.md#authorization-and-grant)
    - [Built-in Authorizations](01_concepts.md#built-in-authorizations)
    - [Gas](01_concepts.md#gas)
2. **[State](02_state.md)**
3. **[Messages](03_messages.md)**
    - [MsgGrant](03_messages.md#msggrant)
    - [MsgRevoke](03_messages.md#msgrevoke)
    - [MsgExec](03_messages.md#msgexec)
4. **[Events](04_events.md)**
