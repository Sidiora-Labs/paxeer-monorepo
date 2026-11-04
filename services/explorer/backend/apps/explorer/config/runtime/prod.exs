import Config

# The umbrella runtime loader requires this file after config/runtime/prod.exs.
# Evaluate the shared policy again so CA mounts are selected at startup.
Code.eval_file(Path.expand("../database_tls.exs", __DIR__))

