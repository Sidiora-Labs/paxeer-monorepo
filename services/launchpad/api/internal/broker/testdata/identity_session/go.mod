module github.com/Sidiora-Technologies/KindleLaunch/media/gateway/snapshot_identity_session

go 1.27.0

require (
 github.com/Sidiora-Technologies/KindleLaunch/media/gateway v0.0.0
 github.com/Sidiora-Technologies/KindleLaunch/shared v0.0.0
 github.com/ethereum/go-ethereum v1.17.4
 github.com/go-chi/chi/v5 v5.4.0
 github.com/jackc/pgx/v5 v5.11.0
)

replace github.com/Sidiora-Technologies/KindleLaunch/shared => ../../../../../../../sdk/launchpad/shared
replace github.com/Sidiora-Technologies/KindleLaunch/protocol => ../../../../../../../sdk/launchpad/protocol
replace github.com/Sidiora-Technologies/KindleLaunch/security/control => ../../../../../security/control
replace github.com/Sidiora-Technologies/KindleLaunch/media/gateway => ../../../../../gateway
