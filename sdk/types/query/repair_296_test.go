package query_test

import (
	"context"
	"encoding/binary"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/query"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/authz"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/types"
)

func TestRepair_296_CallbackSparseContinuation(t *testing.T) {
	app, ctx, _ := setupTest(t)
	last := query.MaxScanLimit + 6
	for id := uint64(1); id <= last; id++ {
		proposal, err := govtypes.NewProposal(govtypes.NewTextProposal("pagination", "sparse proposals", false), id, ctx.BlockTime(), ctx.BlockTime().Add(time.Hour), false)
		require.NoError(t, err)
		proposal.Status = govtypes.StatusRejected
		if id <= 2 || id >= last-1 {
			proposal.Status = govtypes.StatusPassed
		}
		app.GovKeeper.SetProposal(ctx, proposal)
	}

	router := baseapp.NewQueryServerTestHelper(ctx, app.InterfaceRegistry())
	govtypes.RegisterQueryServer(router, app.GovKeeper)
	client := govtypes.NewQueryClient(router)

	for _, reverse := range []bool{false, true} {
		for _, offset := range []uint64{0, 1} {
			t.Run(fmt.Sprintf("reverse=%t/offset=%d", reverse, offset), func(t *testing.T) {
				want := []uint64{1, 2, last - 1, last}
				if reverse {
					want = []uint64{last, last - 1, 2, 1}
				}
				want = want[offset:]
				page := &query.PageRequest{Limit: 2 - offset, Offset: offset, Reverse: reverse}
				var got []uint64
				for n := 0; ; n++ {
					require.Less(t, n, 6, "pagination must make progress")
					res, err := client.Proposals(context.Background(), &govtypes.QueryProposalsRequest{
						ProposalStatus: govtypes.StatusPassed, Pagination: page,
					})
					require.NoError(t, err)
					require.NotNil(t, res.Pagination)
					for _, proposal := range res.Proposals {
						got = append(got, proposal.ProposalId)
					}
					if n == 0 {
						require.Len(t, res.Proposals, int(page.Limit))
						require.NotEmpty(t, res.Pagination.NextKey, "the later matching proposals remain reachable")
						continuationID := binary.BigEndian.Uint64(res.Pagination.NextKey)
						require.Greater(t, continuationID, uint64(2), "continuation must preserve an unprocessed raw key")
						require.Less(t, continuationID, last-1)
					}
					require.Zero(t, res.Pagination.Total)
					if len(res.Pagination.NextKey) == 0 {
						break
					}
					page = &query.PageRequest{Key: res.Pagination.NextKey, Limit: page.Limit, Reverse: reverse}
				}
				require.Equal(t, want, got, "following wire continuations must return every matching proposal exactly once")

				_, err := client.Proposals(context.Background(), &govtypes.QueryProposalsRequest{
					ProposalStatus: govtypes.StatusPassed,
					Pagination:     &query.PageRequest{Limit: 2 - offset, Offset: offset, Reverse: reverse, CountTotal: true},
				})
				require.ErrorContains(t, err, fmt.Sprintf("scanned more than %d entries past the end of the page", query.MaxScanLimit))
			})
		}
	}
}

func TestRepair_296_GenericSparseContinuation(t *testing.T) {
	app, ctx, _ := setupTest(t)
	last := query.MaxScanLimit + 6
	target := sdk.AccAddress(make([]byte, 20))
	target[19] = 1
	other := sdk.AccAddress(make([]byte, 20))
	other[19] = 2
	granters := make([]string, last+1)
	for id := uint64(1); id <= last; id++ {
		granter := sdk.AccAddress(make([]byte, 20))
		binary.BigEndian.PutUint64(granter[12:], id)
		granters[id] = granter.String()
		grantee := other
		if id <= 2 || id >= last-1 {
			grantee = target
		}
		require.NoError(t, app.AuthzKeeper.SaveGrant(ctx, grantee, granter,
			authz.NewGenericAuthorization(sdk.MsgTypeURL(&banktypes.MsgSend{})), ctx.BlockTime().Add(time.Hour)))
	}

	router := baseapp.NewQueryServerTestHelper(ctx, app.InterfaceRegistry())
	authz.RegisterQueryServer(router, app.AuthzKeeper)
	client := authz.NewQueryClient(router)

	for _, reverse := range []bool{false, true} {
		for _, offset := range []uint64{0, 1} {
			t.Run(fmt.Sprintf("reverse=%t/offset=%d", reverse, offset), func(t *testing.T) {
				want := []string{granters[1], granters[2], granters[last-1], granters[last]}
				if reverse {
					want = []string{granters[last], granters[last-1], granters[2], granters[1]}
				}
				want = want[offset:]
				page := &query.PageRequest{Limit: 2 - offset, Offset: offset, Reverse: reverse}
				var got []string
				for n := 0; ; n++ {
					require.Less(t, n, 6, "pagination must make progress")
					res, err := client.GranteeGrants(context.Background(), &authz.QueryGranteeGrantsRequest{
						Grantee: target.String(), Pagination: page,
					})
					require.NoError(t, err)
					require.NotNil(t, res.Pagination)
					for _, grant := range res.Grants {
						require.Equal(t, target.String(), grant.Grantee)
						var decoded authz.Authorization
						require.NoError(t, app.InterfaceRegistry().UnpackAny(grant.Authorization, &decoded))
						authorization, ok := grant.Authorization.GetCachedValue().(authz.Authorization)
						require.True(t, ok)
						require.Equal(t, sdk.MsgTypeURL(&banktypes.MsgSend{}), authorization.MsgTypeURL())
						got = append(got, grant.Granter)
					}
					if n == 0 {
						require.Len(t, res.Grants, int(page.Limit))
						require.NotEmpty(t, res.Pagination.NextKey, "the later matching grants remain reachable")
						continuationID := binary.BigEndian.Uint64(res.Pagination.NextKey[13:21])
						require.Greater(t, continuationID, uint64(2))
						require.Less(t, continuationID, last-1)
					}
					require.Zero(t, res.Pagination.Total)
					if len(res.Pagination.NextKey) == 0 {
						break
					}
					page = &query.PageRequest{Key: res.Pagination.NextKey, Limit: page.Limit, Reverse: reverse}
				}
				require.Equal(t, want, got, "following wire continuations must return every matching grant exactly once")

				_, err := client.GranteeGrants(context.Background(), &authz.QueryGranteeGrantsRequest{
					Grantee:    target.String(),
					Pagination: &query.PageRequest{Limit: 2 - offset, Offset: offset, Reverse: reverse, CountTotal: true},
				})
				require.ErrorContains(t, err, fmt.Sprintf("scanned more than %d entries past the end of the page", query.MaxScanLimit))
			})
		}
	}
}
