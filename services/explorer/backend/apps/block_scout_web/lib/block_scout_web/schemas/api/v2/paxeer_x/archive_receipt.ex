defmodule BlockScoutWeb.Schemas.API.V2.PaxeerX.ArchiveReceipt do
  require OpenApiSpex

  alias BlockScoutWeb.Schemas.API.V2.General
  alias BlockScoutWeb.Schemas.Helper
  alias OpenApiSpex.Schema

  OpenApiSpex.schema(
    %{
      title: "PaxeerXArchiveReceipt",
      description:
        "One kernel receipt as the newest log of its id reports it: the receipt id, the kernel account " <>
          "it belongs to and the settlement rung of the block it was last seen in.",
      type: :object,
      properties: %{
        verification_status: %Schema{
          title: "PaxeerXVerificationStatus",
          description: "The rung the receipt has reached on the LayerX kernel verification lattice.",
          type: :string,
          enum: Enum.map(Explorer.Chain.PaxeerX.Receipt.statuses(), &to_string/1),
          nullable: false
        },
        payload_hash: General.FullHashNullable,
        transaction_hash: General.FullHashNullable,
        timestamp: %Schema{type: :string, format: :"date-time", nullable: true},
        id: General.FullHash,
        account: General.FullHashNullable,
        block_number: %Schema{type: :integer, nullable: true, description: "EVM block, absent for kernel provenance"},
        origin: %Schema{type: :string, enum: ["kernel", "evm"]},
        verification: %Schema{
          type: :string,
          enum: [
            "sequencer_verified",
            "unverified",
            "sequencer_signed",
            "batch_included",
            "state_proven",
            "checkpoint_finalised",
            "settlement_anchored"
          ]
        },
        provenance: %Schema{
          oneOf: [
            %Schema{
              type: :object,
              additionalProperties: false,
              required: [:kernel],
              properties: %{
                kernel: %Schema{
                  type: :object,
                  additionalProperties: false,
                  required: [
                    :batch_number,
                    :batch_id,
                    :sequence,
                    :activity_id,
                    :result_code,
                    :receipt_sha256,
                    :canonical_sha256,
                    :batch_raw_sha256,
                    :state_root,
                    :proof
                  ],
                  properties: %{
                    batch_number: %Schema{type: :string, pattern: "^(0|[1-9][0-9]*)$"},
                    batch_id: General.FullHash,
                    sequence: %Schema{type: :string, pattern: "^(0|[1-9][0-9]*)$"},
                    activity_id: General.FullHash,
                    result_code: %Schema{type: :integer},
                    receipt_sha256: General.FullHash,
                    canonical_sha256: General.FullHash,
                    batch_raw_sha256: General.FullHash,
                    state_root: General.FullHash,
                    proof: %Schema{
                      type: :object,
                      additionalProperties: false,
                      required: [:header_hex, :signature_hex],
                      properties: %{
                        header_hex: %Schema{type: :string, pattern: "^[0-9a-f]+$"},
                        signature_hex: %Schema{type: :string, pattern: "^[0-9a-f]{128}$"}
                      }
                    }
                  }
                }
              }
            },
            %Schema{
              type: :object,
              additionalProperties: false,
              required: [:evm],
              properties: %{
                evm: %Schema{
                  type: :object,
                  additionalProperties: false,
                  required: [:transaction_hash, :log_index, :block_hash, :block_number],
                  properties: %{
                    transaction_hash: General.FullHash,
                    log_index: %Schema{type: :integer, minimum: 0},
                    block_hash: General.FullHash,
                    block_number: %Schema{type: :integer, minimum: 0}
                  }
                }
              }
            }
          ]
        }
      },
      required: [
        :id,
        :account,
        :block_number,
        :origin,
        :verification,
        :provenance,
        :verification_status,
        :payload_hash,
        :transaction_hash,
        :timestamp
      ],
      nullable: false,
      additionalProperties: false
    }
    |> Helper.extend_schema(
      title: "PaxeerXArchiveReceipt",
      description: "An archived receipt with its provenance, verification details and a required frozen marker.",
      properties: %{frozen: %Schema{type: :boolean, enum: [true], nullable: false}},
      required: [:frozen]
    )
  )
end
