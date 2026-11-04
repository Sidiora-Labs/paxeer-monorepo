The systemd show/journal fixtures are peer transcripts for the production
upgrade-observe input protocol. {{PID}} binds the active transcript to the
actual Go gate process; the producer reads its actual /proc stat and executable
bytes. No systemd or SSH command is replaced in the gate. Native RPC records
are reused unchanged from repair_22_31_355/native_configuration.json. Negative
cases alter only the peer responses they are intended to reject.
