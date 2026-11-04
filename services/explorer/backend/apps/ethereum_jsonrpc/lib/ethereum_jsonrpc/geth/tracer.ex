defmodule EthereumJSONRPC.Geth.Tracer do
  @moduledoc """
  Elixir implementation of a custom tracer (`priv/js/ethereum_jsonrpc/geth/debug_traceTransaction/tracer.js`)
  for variants that don't support specifying tracer in [debug_traceTransaction](https://github.com/ethereum/go-ethereum/wiki/Management-APIs#debug_tracetransaction) calls.
  """

  import EthereumJSONRPC, only: [integer_to_quantity: 1, quantity_to_integer: 1]

  def replay(
        %{"structLogs" => logs, "gas" => top_call_gas, "returnValue" => return_value} = result,
        receipt,
        transaction
      )
      when is_list(logs) do
    %{"contractAddress" => contract_address} = receipt
    %{"from" => from, "to" => to, "value" => value, "input" => input} = transaction

    top =
      to
      |> if do
        %{
          "type" => "call",
          "callType" => "call",
          "to" => to,
          "input" => input,
          "output" => Map.get(result, "return", "0x" <> Map.get(result, "returnValue", ""))
        }
      else
        %{
          "type" => "create",
          "init" => input,
          "createdContractAddressHash" => contract_address,
          "createdContractCode" => "0x" <> return_value
        }
      end
      |> Map.merge(%{
        "from" => from,
        "traceAddress" => [],
        "value" => value,
        "gas" => 0,
        "gasUsed" => 0
      })

    ctx = %{
      depth: 1,
      stack: [top],
      trace_address: [0],
      calls: [[]],
      descended: false
    }

    logs
    |> Enum.reduce(ctx, &step/2)
    |> finalize(top_call_gas)
  end

  defp step(
         %{"error" => error, "depth" => depth},
         %{depth: depth, stack: [%{"type" => type} = call | stack]} = ctx
       )
       when type in ~w(create create2) do
    %{ctx | stack: [Map.put(call, "error", error) | stack], descended: false}
  end

  defp step(
         %{"depth" => log_depth} = log,
         %{depth: stack_depth, stack: [%{"type" => type} | _]} = ctx
       )
       when type in ~w(create create2) and log_depth == stack_depth - 1 do
    return_to_parent(log, %{ctx | descended: false})
  end

  defp step(%{"error" => _}, %{stack: [%{"error" => _} | _]} = ctx), do: ctx

  defp step(
         %{"error" => _} = log,
         %{
           depth: stack_depth,
           stack: [call | stack],
           trace_address: [_, trace_index | trace_address],
           calls: [subsubcalls, subcalls | calls]
         } = ctx
       ) do
    call = process_return(log, Map.put(call, "error", "error"))

    subsubcalls =
      subsubcalls
      |> Enum.reverse()
      |> Enum.map(fn
        subcalls when is_list(subcalls) -> subcalls
        subcall when is_map(subcall) -> %{subcall | "from" => call["createdContractAddressHash"] || call["to"]}
      end)

    %{
      ctx
      | depth: stack_depth - 1,
        stack: stack,
        trace_address: [trace_index + 1 | trace_address],
        calls: [[subsubcalls, call | subcalls] | calls]
    }
  end

  defp step(
         %{"gas" => log_gas} = log,
         %{
           stack: [%{"gas" => call_gas} = call | stack],
           descended: true
         } = ctx
       ) do
    gas = max(call_gas, log_gas)
    call = %{call | "gas" => gas}
    step(log, %{ctx | stack: [call | stack], descended: false})
  end

  defp step(%{"depth" => log_depth} = log, %{depth: stack_depth} = ctx)
       when log_depth == stack_depth - 1 do
    return_to_parent(log, ctx)
  end

  defp step(%{"gas" => log_gas, "gasCost" => log_gas_cost} = log, %{stack: [%{"gas" => call_gas} = call | stack]} = ctx) do
    gas = max(call_gas, log_gas)
    op(log, %{ctx | stack: [%{call | "gas" => gas, "gasUsed" => gas - log_gas - log_gas_cost} | stack]})
  end

  defp return_to_parent(
         log,
         %{
           depth: stack_depth,
           stack: [call | stack],
           trace_address: [_, trace_index | trace_address],
           calls: [subsubcalls, subcalls | calls]
         } = ctx
       ) do
    call = process_return(log, call)

    subsubcalls =
      subsubcalls
      |> Enum.reverse()
      |> Enum.map(fn
        subcalls when is_list(subcalls) -> subcalls
        subcall when is_map(subcall) -> %{subcall | "from" => call["createdContractAddressHash"] || call["to"]}
      end)

    step(log, %{
      ctx
      | depth: stack_depth - 1,
        stack: stack,
        trace_address: [trace_index + 1 | trace_address],
        calls: [[subsubcalls, call | subcalls] | calls]
    })
  end

  defp op(%{"op" => "CREATE"} = log, ctx), do: create_op(log, ctx)
  defp op(%{"op" => "CREATE2"} = log, ctx), do: create_op(log, ctx)
  defp op(%{"op" => "SELFDESTRUCT"} = log, ctx), do: self_destruct_op(log, ctx)
  defp op(%{"op" => "CALL"} = log, ctx), do: call_op(log, "call", ctx)
  defp op(%{"op" => "CALLCODE"} = log, ctx), do: call_op(log, "callcode", ctx)
  defp op(%{"op" => "DELEGATECALL"} = log, ctx), do: call_op(log, "delegatecall", ctx)
  defp op(%{"op" => "STATICCALL"} = log, ctx), do: call_op(log, "staticcall", ctx)

  defp op(
         %{"op" => "RETURN", "stack" => log_stack} = log,
         %{stack: [%{"type" => type} = call | stack]} = ctx
       )
       when type in ~w(create create2) do
    [offset, length | _] = Enum.reverse(log_stack)
    output = memory_slice(log, offset, length)
    %{ctx | stack: [%{call | "createdContractCode" => "0x" <> output} | stack]}
  end

  defp op(%{"op" => "REVERT"}, ctx), do: revert_op(ctx)
  defp op(_, ctx), do: ctx

  defp process_return(
         %{"stack" => log_stack, "gas" => log_gas},
         %{"type" => type, "parentGas" => parent_gas} = call
       )
       when type in ~w(create create2) do
    [ret | _] = Enum.reverse(log_stack)
    call = %{call | "gasUsed" => parent_gas - log_gas}

    ret
    |> quantity_to_integer()
    |> case do
      0 ->
        call
        |> Map.put("error", call["error"] || "internal failure")
        |> Map.delete("createdContractAddressHash")
        |> Map.put("createdContractCode", "0x")

      address ->
        address_hash = "0x" <> (address |> Integer.to_string(16) |> String.downcase() |> String.pad_leading(40, "0"))
        %{call | "createdContractAddressHash" => address_hash}
    end
    |> Map.delete("parentGas")
  end

  defp process_return(
         %{"stack" => log_stack, "memory" => log_memory},
         %{"outputOffset" => out_off, "outputLength" => out_len} = call
       ) do
    [ret | _] = Enum.reverse(log_stack)

    ret
    |> quantity_to_integer()
    |> case do
      0 ->
        Map.put(call, "error", call["error"] || "internal failure")

      _ ->
        output =
          log_memory
          |> IO.iodata_to_binary()
          |> String.slice(out_off, out_len)

        %{call | "output" => "0x" <> output}
    end
    |> Map.drop(["outputOffset", "outputLength"])
  end

  defp create_op(
         %{"op" => opcode, "stack" => log_stack, "gas" => log_gas, "gasCost" => log_gas_cost} = log,
         %{depth: stack_depth, stack: stack, trace_address: trace_address, calls: calls} = ctx
       ) do
    {type, value, input_offset, input_length} =
      case {opcode, Enum.reverse(log_stack)} do
        {"CREATE", [value, offset, length | _]} -> {"create", value, offset, length}
        {"CREATE2", [value, offset, length, _salt | _]} -> {"create2", value, offset, length}
      end

    parent_gas = log_gas - log_gas_cost

    call = %{
      "type" => type,
      "from" => nil,
      "traceAddress" => Enum.reverse(trace_address),
      "init" => "0x" <> memory_slice(log, input_offset, input_length),
      "gas" => parent_gas - div(parent_gas, 64),
      "gasUsed" => 0,
      "value" => value,
      "parentGas" => parent_gas,
      "createdContractAddressHash" => nil,
      "createdContractCode" => "0x"
    }

    %{
      ctx
      | depth: stack_depth + 1,
        stack: [call | stack],
        trace_address: [0 | trace_address],
        calls: [[] | calls],
        descended: true
    }
  end

  defp memory_slice(log, offset, length) do
    length = quantity_to_integer(length) * 2

    if length == 0 do
      ""
    else
      log
      |> Map.get("memory", [])
      |> IO.iodata_to_binary()
      |> String.slice(quantity_to_integer(offset) * 2, length)
      |> String.pad_trailing(length, "0")
    end
  end

  defp self_destruct_op(
         %{"stack" => log_stack, "gas" => log_gas, "gasCost" => log_gas_cost},
         %{trace_address: [trace_index | trace_address], calls: [subcalls | calls]} = ctx
       ) do
    [to | _] = Enum.reverse(log_stack)

    if quantity_to_integer(to) in 1..8 do
      ctx
    else
      call = %{
        "type" => "selfdestruct",
        "from" => nil,
        "to" => to,
        "traceAddress" => Enum.reverse([trace_index | trace_address]),
        "gas" => log_gas,
        "gasUsed" => log_gas_cost,
        "value" => "0x0"
      }

      %{ctx | trace_address: [trace_index + 1 | trace_address], calls: [[call | subcalls] | calls]}
    end
  end

  defp call_op(
         %{"stack" => log_stack, "memory" => log_memory},
         call_type,
         %{
           depth: stack_depth,
           stack: [%{"value" => parent_value} = parent | stack],
           trace_address: trace_address,
           calls: calls
         } = ctx
       ) do
    [_, to | log_stack] = Enum.reverse(log_stack)

    {value, [input_offset, input_length, output_offset, output_length | _]} =
      case call_type do
        "delegatecall" ->
          {parent_value, log_stack}

        "staticcall" ->
          {"0x0", log_stack}

        _ ->
          [value | rest] = log_stack
          {value, rest}
      end

    input =
      log_memory
      |> IO.iodata_to_binary()
      |> String.slice(quantity_to_integer(input_offset) * 2, quantity_to_integer(input_length) * 2)

    call = %{
      "type" => "call",
      "callType" => call_type,
      "from" => nil,
      "to" => to,
      "traceAddress" => Enum.reverse(trace_address),
      "input" => "0x" <> input,
      "output" => "0x",
      "outputOffset" => quantity_to_integer(output_offset) * 2,
      "outputLength" => quantity_to_integer(output_length) * 2,
      "gas" => 0,
      "gasUsed" => 0,
      "value" => value
    }

    %{
      ctx
      | depth: stack_depth + 1,
        stack: [call, parent | stack],
        trace_address: [0 | trace_address],
        calls: [[] | calls],
        descended: true
    }
  end

  defp revert_op(%{stack: [last | stack]} = ctx) do
    %{ctx | stack: [Map.put(last, "error", "execution reverted") | stack]}
  end

  defp finalize(%{stack: [top], calls: [calls]}, top_call_gas) do
    calls =
      Enum.map(calls, fn
        subcalls when is_list(subcalls) -> subcalls
        subcall when is_map(subcall) -> %{subcall | "from" => top["createdContractAddressHash"] || top["to"]}
      end)

    [%{top | "gasUsed" => top_call_gas} | Enum.reverse(calls)]
    |> List.flatten()
    |> Enum.map(fn
      %{"type" => type, "gas" => gas, "gasUsed" => gas_used} = call when type in ~w(create create2) ->
        %{
          call
          | "gas" => gas |> integer_to_quantity() |> String.downcase(),
            "gasUsed" => gas_used |> integer_to_quantity() |> String.downcase()
        }

      %{"gas" => gas, "gasUsed" => gas_used} = call ->
        %{call | "gas" => integer_to_quantity(gas), "gasUsed" => integer_to_quantity(gas_used)}
    end)
  end
end
