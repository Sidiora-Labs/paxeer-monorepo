<!--
order: 2
-->

# Subspace

A `Subspace` provides a module with a named prefix in the shared parameter store.
Its implementation is in [`types/subspace.go`](../types/subspace.go). Giving a
keeper its own subspace isolates its parameter keys from those of other modules;
code holding the subspace can use the methods it exposes.

## Key

Parameter keys registered through `KeyTable.RegisterType` must be non-empty,
alphanumeric strings. A key `ExampleParameter` in subspace `SubspaceName` is
stored at `[]byte("SubspaceName/ExampleParameter")` in the persistent `params`
store. Values are encoded using legacy Amino JSON. Writes through `SetRaw`,
including those made by `Set` and `Update`, also record the same prefixed key in
`transient_params`; `Modified` checks this block-local marker.

Keys are exact byte strings. Constructing a longer key from a registered key
does not create an inherited subkey or a nested parameter namespace. Every key
used by the typed accessors or `Update` needs its own registration. Fields in a
stored object, such as the [governance parameters](../../gov/spec/06_params.md),
are decoded as part of that object, not looked up as subkeys.

For example, registering `ExampleParameter` does not register
`ExampleParameterChild`. `ExampleParameterChild` must be registered separately
with its own type and validator through `KeyTable.RegisterType` (or
`RegisterParamSet`). A governance proposal's basic validation does not check
this registration; executing an update for an unregistered key panics.

## KeyTable

[`KeyTable`](../types/table.go) maps each exact key string to an attribute
containing a `reflect.Type` and a `ValueValidatorFn`. Modules normally construct
the table during keeper initialization and attach it with `WithKeyTable`.
Registration rejects duplicate keys, invalid key strings and nil validators.
`WithKeyTable` rejects a nil table or replacement of an already populated table.

`checkType` performs an exact map lookup and panics for an unregistered key or a
type mismatch. `Get` decodes into the supplied pointer and panics on a missing or
undecodable value. `GetIfExists` leaves the destination unchanged when the value
is absent. `Set` checks the registered type and encodes the value; it does not
invoke the registered validator itself.

`Validate` returns an error for an unregistered key or an invalid value.
`Update` panics on an unregistered key, loads the existing value if present,
decodes the supplied JSON into that value, validates the result, and calls `Set`.
This preserves omitted fields when updating an existing object. JSON decoding
and validation failures are returned as errors.

`GetRaw`, `SetRaw` and `Has` operate directly on the prefixed store without the
typed registration checks. They do not provide subkey inheritance or automatic
validation.

## ParamSet

A module's parameter struct can implement the
[`ParamSet`](../types/paramset.go) interface by returning `ParamSetPairs`, each
with a key, field pointer and validator.

- `KeyTable.RegisterParamSet` registers each pair's exact key and type.
- `Subspace.GetParamSet` loads each value into its field pointer.
- `Subspace.GetParamSetIfExists` loads only the values present in the store.
- `Subspace.SetParamSet` validates each dereferenced field, then stores it with
  `Set`; invalid values cause a panic.

Use a pointer to the parameter struct so reads can populate its fields. The
[slashing parameter type](../../slashing/types/params.go) implements this pattern.
