package com.sidiora.layerx.sdk.examples

import com.fasterxml.jackson.databind.node.TextNode
import com.sidiora.layerx.sdk.*
import java.net.URI
import java.nio.charset.StandardCharsets
import java.util.concurrent.CompletionStage

/** An owner-session DaemonLimit protects only calls through this daemon. */
class ServiceLifecycleExample {
    companion object {
        @JvmStatic
        fun createBudget(
            humanOrigin: URI,
            agentOrigin: URI,
            bearer: HttpProductionTransport.BearerCredential,
            ownerSession: HttpProductionTransport.AgentSessionCredential,
            tenant: String,
            agentDid: String,
            asset: String,
            limit: ProtocolAmount,
            expirySeconds: String,
            key: IdempotencyKey,
        ): CompletionStage<GeneratedSchema.AgentOperations.BudgetCreateResponse> {
            val client = ProductionClient(
                HttpProductionTransport.create(
                    ExampleConfiguration.serviceOrigin(humanOrigin.toString()),
                    ExampleConfiguration.serviceOrigin(agentOrigin.toString()),
                    bearer,
                    ownerSession,
                ),
            )
            val request = GeneratedSchema.AgentOperations.BudgetCreateRequest(
                TextNode.valueOf(tenant),
                TextNode.valueOf(agentDid),
                TextNode.valueOf(asset),
                TextNode.valueOf(limit.toString()),
                TextNode.valueOf("DaemonLimit"),
                TextNode.valueOf(expirySeconds),
            )
            return client.agent(GeneratedSchema.AgentOperations.BUDGET_CREATE, request, ProductionClient.Options.idempotent(key))
        }

        @JvmStatic
        fun main(args: Array<String>) {
            val key = ExampleConfiguration.randomKey()
            println("Budget idempotency reference: ${key.value()}")
            HttpProductionTransport.BearerCredential(
                SecretBytes(ExampleConfiguration.required("LAYERX_API_KEY").toByteArray(StandardCharsets.UTF_8)),
            ).use { bearer ->
                val tenant = ExampleConfiguration.required("LAYERX_TENANT")
                ExampleConfiguration.session(
                    tenant,
                    ExampleConfiguration.required("LAYERX_SESSION_ID"),
                    ExampleConfiguration.required("LAYERX_SESSION_TOKEN_ID"),
                    ExampleConfiguration.required("LAYERX_SESSION_GENERATION"),
                ).use { ownerSession ->
                    val result = createBudget(
                        ExampleConfiguration.serviceOrigin(ExampleConfiguration.required("LAYERX_HUMAN_ORIGIN")),
                        ExampleConfiguration.serviceOrigin(ExampleConfiguration.required("LAYERX_AGENT_ORIGIN")),
                        bearer,
                        ownerSession,
                        tenant,
                        ExampleConfiguration.required("LAYERX_AGENT_DID"),
                        ExampleConfiguration.required("LAYERX_ASSET"),
                        ProtocolAmount.parse(ExampleConfiguration.required("LAYERX_BUDGET_LIMIT")),
                        ExampleConfiguration.required("LAYERX_BUDGET_EXPIRY_SECONDS"),
                        key,
                    ).toCompletableFuture().join()
                    println("Budget created: ${result.value().required("budget_id").textValue()}")
                    println(result.value().required("notice").textValue())
                }
            }
        }
    }
}
