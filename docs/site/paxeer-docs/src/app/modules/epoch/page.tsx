import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/modules/epoch')

export default function Epoch() {
  return (
    <DocsLayout>
      <div className="page-header">
        <h1 className="page-title">Epoch Module</h1>
        <p className="page-description">
          Time-based hooks and epoch lifecycle management for coordinating periodic chain operations.
        </p>
      </div>

      <div className="source-note">
        <strong>Source:</strong> <code>Paxeer-X-Network/modules/epoch/</code>
      </div>

      <h2>Overview</h2>

      <p>
        The epoch module manages fixed time periods (epochs) and triggers registered hooks when an epoch ends and the next begins. An epoch defaults to <strong>60 seconds</strong> (<code>time.Minute</code> in genesis). Hooks run during <code>BeginBlock</code>.
      </p>

      <p>
        In this codebase only one hook consumer is wired in <code>node/app.go</code>: the mint keeper. Validator-set updates, parameter activation, and periodic cleanup are not epoch-hook consumers here.
      </p>

      <h2>State</h2>

      <p>
        The epoch module maintains a single state object:
      </p>

      <pre><code>{`> paxd q epoch epoch --output json
{
  "epoch": {
    "genesis_time": "2023-04-27T19:08:11.958027Z",
    "epoch_duration": "60s",
    "current_epoch": "0",
    "current_epoch_start_time": "2023-04-27T19:08:11.958027Z",
    "current_epoch_height": "0"
  }
}`}</code></pre>

      <ul>
        <li><strong>genesis_time:</strong> Chain genesis time (fixed)</li>
        <li><strong>epoch_duration:</strong> Duration of each epoch (default 60s)</li>
        <li><strong>current_epoch:</strong> Current epoch number (increments at each epoch boundary)</li>
        <li><strong>current_epoch_start_time:</strong> Timestamp when the current epoch started</li>
        <li><strong>current_epoch_height:</strong> Block height at which the current epoch started</li>
      </ul>

      <h2>Epoch Boundaries</h2>

      <p>
        Epochs are defined by wall-clock time, not block height. In <code>BeginBlock</code> (<code>modules/epoch/keeper/abci.go</code>) a new epoch begins only when the block time is <em>strictly after</em> the boundary:
      </p>

      <pre><code>{`if ctx.BlockTime().Sub(lastEpoch.CurrentEpochStartTime) > lastEpoch.EpochDuration {
    // advance exactly one epoch
}`}</code></pre>

      <ul>
        <li>The comparison is strict (<code>&gt;</code>), not <code>≥</code>.</li>
        <li>It is a single <code>if</code>, not a loop: at most one epoch advance per block, even after a long halt.</li>
        <li>The new start time is <code>oldStart + epoch_duration</code>, not the current block time, so the schedule does not drift to block time.</li>
      </ul>

      <h2>Hooks</h2>

      <p>
        Other modules implement the <code>EpochHooks</code> interface (<code>modules/epoch/types/hooks.go</code>):
      </p>

      <pre><code>{`type EpochHooks interface {
    AfterEpochEnd(ctx sdk.Context, epoch Epoch)
    BeforeEpochStart(ctx sdk.Context, epoch Epoch)
}`}</code></pre>

      <p>At a boundary the keeper runs them in this order:</p>

      <ol>
        <li><code>AfterEpochEnd(ctx, lastEpoch)</code> — for the epoch that is ending</li>
        <li><code>SetEpoch(newEpoch)</code> — store the incremented epoch</li>
        <li><code>BeforeEpochStart(ctx, newEpoch)</code> — for the epoch that is starting</li>
        <li>Emit the <code>new_epoch</code> event</li>
      </ol>

      <p>
        Each hook runs in a cached context (<code>ctx.CacheContext()</code>). Its writes are committed only if the hook completes; a hook panic is re-raised and aborts the block rather than advancing the epoch without the hook&apos;s state transition.
      </p>

      <h3>Example: Mint Module</h3>

      <p>
        The mint module (<code>modules/mint/keeper/hooks.go</code>) implements epoch hooks: <code>BeforeEpochStart</code> is a no-op and <code>AfterEpochEnd</code> performs the daily release check. It is the only hook registered in <code>node/app.go</code>:
      </p>

      <pre><code>{`app.EpochKeeper = *epochmodulekeeper.NewKeeper(
  appCodec,
  keys[epochmoduletypes.StoreKey],
  keys[epochmoduletypes.MemStoreKey],
  app.GetSubspace(epochmoduletypes.ModuleName),
).SetHooks(epochmoduletypes.NewMultiEpochHooks(
  app.MintKeeper.Hooks()))`}</code></pre>

      <h2>Events</h2>

      <p>
        The epoch module emits a <code>new_epoch</code> event at each epoch boundary:
      </p>

      <ul>
        <li><strong>epoch_number:</strong> The new epoch&apos;s epoch number</li>
        <li><strong>epoch_time:</strong> The new epoch&apos;s start time</li>
        <li><strong>epoch_height:</strong> The block height at which the new epoch started</li>
      </ul>

      <h2>Messages</h2>

      <p>
        The epoch module does not expose any transactions. Its handler rejects every message as unrecognized. Only the module itself updates epoch state during <code>BeginBlock</code>.
      </p>

      <h2>Parameters</h2>

      <p>
        The epoch module has no runtime parameters: <code>Params</code> is empty and validation rejects any non-empty value. The epoch duration lives in the epoch genesis state.
      </p>

      <h2>Use Cases</h2>

      <ul>
        <li><strong>Mint module:</strong> The only wired consumer; its <code>AfterEpochEnd</code> hook checks whether a daily release is due. See <Link href="/modules/mint">Mint module</Link></li>
      </ul>

      <h2>Next Steps</h2>

      <ul>
        <li><Link href="/modules/mint">Understand how the mint module uses epochs</Link></li>
        <li><Link href="/modules">Review all Paxeer modules</Link></li>
      </ul>

      <div className="prev-next">
        <Link href="/modules">
          <div className="prev-next-label">Previous</div>
          <div className="prev-next-title">Modules Overview</div>
        </Link>
        <Link href="/modules/mint">
          <div className="prev-next-label">Next</div>
          <div className="prev-next-title">Mint Module</div>
        </Link>
      </div>
    </DocsLayout>
  )
}
