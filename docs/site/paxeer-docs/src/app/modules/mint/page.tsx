import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/modules/mint')

export default function Mint() {
  return (
    <DocsLayout>
      <div className="page-header">
        <h1 className="page-title">Mint Module</h1>
        <p className="page-description">
          Scheduled native token release: a fixed amount minted daily through the epoch hook, under governance-controlled schedules.
        </p>
      </div>

      <div className="source-note">
        <strong>Source:</strong> <code>Paxeer-X-Network/modules/mint/</code>
      </div>

      <h2>Overview</h2>

      <p>
        The mint module creates new native tokens according to a predefined schedule: a <code>total_mint_amount</code> released between a start date and an end date. There is no block-by-block inflation rate. Minting is driven only by the epoch module&apos;s <code>AfterEpochEnd</code> hook and releases at most once per UTC date.
      </p>

      <h2>Minting Mechanism</h2>

      <p>
        On each epoch end, the hook (<code>modules/mint/keeper/hooks.go</code>) loads or updates the current minter from the release schedule, then computes today&apos;s release from the <em>ending epoch&apos;s start time</em> — not the current block time:
      </p>

      <pre><code>{`coinsToMint := latestMinter.GetReleaseAmountToday(epoch.CurrentEpochStartTime.UTC())`}</code></pre>

      <p><code>GetReleaseAmountToday</code> (<code>modules/mint/types/minter.go</code>) applies these rules, in order:</p>

      <ol>
        <li>Before the start date, or if a mint already happened on that UTC date (<code>last_mint_date</code> matches): release <code>0</code></li>
        <li>Past the end date, or with zero days left: release <strong>all</strong> of <code>remaining_mint_amount</code></li>
        <li>Otherwise: release <code>remaining_mint_amount / days_between(today, end_date)</code> (integer division)</li>
      </ol>

      <p>
        If the amount is zero or nothing remains, the hook returns without minting. Otherwise it mints the coins (<code>MintCoins</code>), sends them to the <code>fee_collector</code> account (<code>AddCollectedFees</code>), and records the mint: <code>remaining_mint_amount</code> decreases and <code>last_mint_amount</code>, <code>last_mint_date</code>, and <code>last_mint_height</code> are updated. The distribution module then handles collected fees as usual.
      </p>

      <h3>Example</h3>

      <p>
        If <code>total_mint_amount = 1,000,000</code> tokens and the period is 100 days:
      </p>

      <ul>
        <li>Initial daily mint: <code>1,000,000 / 100 = 10,000</code> tokens/day</li>
        <li>After 50 days: <code>500,000 remaining / 50 days = 10,000</code> tokens/day (unchanged if no downtime)</li>
        <li>If the chain was down for 1 day at day 50: <code>500,000 remaining / 49 days = 10,204</code> tokens/day (adjusted)</li>
        <li>If the chain is still down past the end date, the first epoch end after recovery releases the entire remainder at once (rule 2)</li>
      </ul>

      <h2>State</h2>

      <h3>Minter</h3>

      <p>
        The minter stores current release state. Dates use the <code>yyyy-mm-dd</code> format:
      </p>

      <pre><code>{`type Minter struct {
    StartDate           string  // The day where the mint begins
    EndDate             string  // The day where the mint ends
    Denom               string  // Must equal the bond denom (uhpx)
    TotalMintAmount     uint64  // Total amount to be minted
    RemainingMintAmount uint64  // Remaining amount to be minted
    LastMintAmount      uint64  // Last amount minted (usually from the day before)
    LastMintDate        string  // Last day minted
    LastMintHeight      uint64  // The height of the last mint
}`}</code></pre>

      <p>
        The denom is not a free choice: validation requires both the minter denom and <code>Params.MintDenom</code> to equal the default bond denom, <code>uhpx</code>.
      </p>

      <p>
        Query the current minter and parameters:
      </p>

      <pre><code>{`paxd q mint minter
paxd q mint params`}</code></pre>

      <h3>Params</h3>

      <p>
        Parameters define the release schedules the minter is derived from:
      </p>

      <pre><code>{`type Params struct {
    MintDenom            string                   // Type of coin to mint (must be uhpx)
    TokenReleaseSchedule []ScheduledTokenRelease  // List of token release schedules
}

type ScheduledTokenRelease struct {
    StartDate          string  // The day where the mint begins
    EndDate            string  // The day where the mint ends
    TokenReleaseAmount uint64  // Total amount to be minted
}`}</code></pre>

      <p>
        Multiple release schedules can be defined, allowing sequential minting periods.
      </p>

      <h2>Epoch Hook</h2>

      <p>
        The mint module has no begin-block minting logic of its own. Its <code>BeforeEpochStart</code> hook is a no-op; all minting happens in <code>AfterEpochEnd</code> as described above. Because epochs default to 60s, the hook runs frequently, but the same-date check limits actual mints to one per UTC date. See the <Link href="/modules/epoch">Epoch module</Link> for how boundaries are detected.
      </p>

      <h2>Governance</h2>

      <h3>Update Minter Proposal</h3>

      <p>
        The minter can be replaced via governance to change the mint schedule. The proposal is stored <em>verbatim</em> after validation (<code>HandleUpdateMinterProposal</code> calls <code>ValidateMinter</code> then <code>SetMinter</code>), so every field must be set explicitly. In particular:
      </p>

      <ul>
        <li><code>remaining_mint_amount</code> is not derived from <code>total_mint_amount</code>. Setting it to <code>0</code> means no ongoing release and no further mints</li>
        <li><code>last_mint_date</code> must be a parseable <code>yyyy-mm-dd</code> date; an empty string fails validation. A fresh minter uses <code>0001-01-01</code></li>
        <li><code>denom</code> must be <code>uhpx</code>, and <code>last_mint_amount</code> must not exceed <code>total_mint_amount - remaining_mint_amount</code></li>
      </ul>

      <pre><code>{`{
  "title": "Update Minter",
  "description": "Adjust mint schedule",
  "minter": {
    "start_date": "2023-10-05",
    "end_date": "2023-11-22",
    "denom": "uhpx",
    "total_mint_amount": "100000",
    "remaining_mint_amount": "100000",
    "last_mint_amount": "0",
    "last_mint_date": "0001-01-01",
    "last_mint_height": "0"
  }
}`}</code></pre>

      <p>
        Submit the proposal:
      </p>

      <pre><code>{`paxd tx gov submit-proposal update-minter ./minter_prop.json \\
  --deposit 20pax --from admin -b block -y \\
  --gas 200000 --fees 2000uhpx`}</code></pre>

      <p>
        After the proposal passes, <code>paxd q mint minter</code> returns exactly the stored minter above until the next daily release updates it.
      </p>

      <h3>Update Params Proposal</h3>

      <p>
        Parameters can be updated to define new token release schedules:
      </p>

      <pre><code>{`{
  "title": "Param Change Proposal",
  "description": "Update token release schedule",
  "changes": [
    {
      "subspace": "mint",
      "key": "MintDenom",
      "value": "uhpx"
    },
    {
      "subspace": "mint",
      "key": "TokenReleaseSchedule",
      "value": [
        {
          "token_release_amount": 500,
          "start_date": "2023-10-01",
          "end_date": "2023-10-30"
        },
        {
          "token_release_amount": 1000,
          "start_date": "2023-11-01",
          "end_date": "2023-11-30"
        }
      ]
    }
  ]
}`}</code></pre>

      <p>
        Submit:
      </p>

      <pre><code>{`paxd tx gov submit-proposal param-change ./param_change_prop.json \\
  --from admin -b block -y --gas 200000 --fees 200000uhpx`}</code></pre>

      <div className="source-note">
        <strong>Note:</strong> Changes to <code>total_mint_amount</code> or <code>remaining_mint_amount</code> after the start date do not affect tokens already minted.
      </div>

      <h2>Events</h2>

      <h3>Mint Event</h3>

      <p>
        Emitted on each successful mint:
      </p>

      <ul>
        <li><strong>mint_date:</strong> Date of the mint</li>
        <li><strong>mint_epoch:</strong> Epoch number when the mint occurred</li>
        <li><strong>amount:</strong> Amount minted</li>
      </ul>

      <h2>Metrics</h2>

      <p>
        Each successful mint records two gauges: the OpenTelemetry gauge <code>mint_coins_minted&#123;denom&#125;</code> and the legacy telemetry gauge <code>pax/mint/coins</code> (exported as <code>pax_mint_coins&#123;denom&#125;</code>).
      </p>

      <h2>Next Steps</h2>

      <ul>
        <li><Link href="/modules/epoch">Understand epoch coordination</Link></li>
        <li><Link href="/modules/oracle">Learn about the oracle module</Link></li>
      </ul>

      <div className="prev-next">
        <Link href="/modules/epoch">
          <div className="prev-next-label">Previous</div>
          <div className="prev-next-title">Epoch Module</div>
        </Link>
        <Link href="/modules/oracle">
          <div className="prev-next-label">Next</div>
          <div className="prev-next-title">Oracle Module</div>
        </Link>
      </div>
    </DocsLayout>
  )
}
