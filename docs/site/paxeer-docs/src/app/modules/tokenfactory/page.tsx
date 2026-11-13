import { metadataFor } from '@/lib/seo'
import { DocsLayout } from '@/components/DocsLayout'
import Link from 'next/link'

export const metadata = metadataFor('/modules/tokenfactory')

export default function TokenFactory() {
  return (
    <DocsLayout>
      <div className="page-header">
        <h1 className="page-title">Token Factory Module</h1>
        <p className="page-description">
          Creation and admin management of native token denominations with creator-namespaced denoms.
        </p>
      </div>

      <div className="source-note">
        <strong>Source:</strong> <code>Paxeer-X-Network/modules/tokenfactory/</code>
      </div>

      <h2>Overview</h2>

      <p>
        The tokenfactory module allows any account to create new native tokens: <code>MsgCreateDenom</code> has no permission gate, allow list of creators, or governance check. Tokens are namespaced by creator address as <code>factory/&#123;creator address&#125;/&#123;subdenom&#125;</code>, eliminating name collisions. A single account can create multiple denoms by providing unique subdenoms.
      </p>

      <p>
        The creator becomes the denom&apos;s admin. The admin can mint, burn, change admin, update the denom allow list, set bank metadata, and pause or unpause the denom — subject to the sender restrictions below.
      </p>

      <h2>Token Naming</h2>

      <p>
        Tokenfactory denoms follow the format:
      </p>

      <pre><code>{`factory/{creator_address}/{subdenom}`}</code></pre>

      <p>
        Example:
      </p>

      <pre><code>{`factory/pax166vhptur29s3gw5qr6dm30s06gej6pr4zevkqk/ufoo`}</code></pre>

      <ul>
        <li><strong>Prefix:</strong> Always <code>factory</code> (7 bytes)</li>
        <li><strong>Creator address:</strong> Bech32 address (max 75 bytes)</li>
        <li><strong>Subdenom:</strong> User-defined (max 44 bytes)</li>
      </ul>

      <p>
        The full denom must also pass the SDK denom validation, <code>[a-zA-Z][a-zA-Z0-9/-]&#123;2,127&#125;</code> (<code>sdk/types/coin.go</code>). That means a subdenom containing <code>.</code> is rejected, while <code>-</code> and <code>/</code> are allowed characters in the full denom. Total denom length must not exceed 128 bytes.
      </p>

      <h2>Messages</h2>

      <h3>CreateDenom</h3>

      <p>
        Create a new denom. Field 3 carries an optional bank allow list for the new denom:
      </p>

      <pre><code>{`message MsgCreateDenom {
    string sender = 1;
    string subdenom = 2;
    AllowList allow_list = 3;  // bank AllowList type
}`}</code></pre>

      <p>
        CLI:
      </p>

      <pre><code>{`paxd tx tokenfactory create-denom ufoo --from mylocalwallet`}</code></pre>

      <p>
        State modifications:
      </p>

      <ul>
        <li>Set <code>DenomMetadata</code> via bank keeper</li>
        <li>Set <code>AuthorityMetadata</code> with sender as admin</li>
        <li>Add denom to <code>CreatorPrefixStore</code></li>
      </ul>

      <p>
        Creation fails if the subdenom equals an existing native denom or the resulting denom already has bank metadata.
      </p>

      <h3>Mint</h3>

      <p>
        Mint tokens (admin only). The minted coins go to the sender — there is no recipient field:
      </p>

      <pre><code>{`message MsgMint {
    string sender = 1;
    cosmos.base.v1beta1.Coin amount = 2;
}`}</code></pre>

      <p>
        CLI:
      </p>

      <pre><code>{`paxd tx tokenfactory mint 100000000000factory/pax166vh.../ufoo --from mylocalwallet`}</code></pre>

      <p>
        Checks: the denom was created via tokenfactory, the sender is the admin, and the denom is not paused. The amount is minted via the bank module to the sender&apos;s account; distributing it further is a separate bank send.
      </p>

      <h3>Burn</h3>

      <p>
        Burn tokens (admin only). The coins are burned from the sender&apos;s own balance — the admin cannot burn from another account:
      </p>

      <pre><code>{`message MsgBurn {
    string sender = 1;
    cosmos.base.v1beta1.Coin amount = 2;
}`}</code></pre>

      <p>
        Checks: the denom was created via tokenfactory, the sender is the admin, the denom is not paused, and the sender holds the amount. The amount is sent from the sender to the module account and burned via the bank module.
      </p>

      <h3>ChangeAdmin</h3>

      <p>
        Transfer admin privileges:
      </p>

      <pre><code>{`message MsgChangeAdmin {
    string sender = 1;
    string denom = 2;
    string newAdmin = 3;
}`}</code></pre>

      <p>
        The sender must be the current admin, and <code>newAdmin</code> must differ from the current admin. <code>ValidateBasic</code> requires <code>newAdmin</code> to be a valid, non-empty bech32 address, so admin cannot be renounced by setting it to <code>""</code> through this message; an empty admin can only exist in stored or genesis authority metadata.
      </p>

      <h3>UpdateDenom</h3>

      <p>
        Replace the denom&apos;s bank allow list:
      </p>

      <pre><code>{`message MsgUpdateDenom {
    string sender = 1;
    string denom = 2;
    AllowList allow_list = 3;  // bank AllowList type
}`}</code></pre>

      <pre><code>{`paxd tx tokenfactory update-denom <denom> --from mylocalwallet`}</code></pre>

      <p>
        The allow list must be defined, contain only valid bech32 addresses, and not exceed the <code>denom_allowlist_max_size</code> parameter (default <code>2000</code>).
      </p>

      <h3>SetDenomMetadata</h3>

      <p>
        Set the denom&apos;s bank metadata from a file:
      </p>

      <pre><code>{`paxd tx tokenfactory set-denom-metadata <metadata-file> --from mylocalwallet`}</code></pre>

      <h3>SetDenomPaused</h3>

      <p>
        Pause or unpause a denom:
      </p>

      <pre><code>{`paxd tx tokenfactory set-denom-paused <denom> true --from mylocalwallet`}</code></pre>

      <p>
        While a denom is paused, tokenfactory mint and burn reject with <code>ErrDenomPaused</code>.
      </p>

      <h2>Admin Capabilities</h2>

      <p>
        The admin can:
      </p>

      <ul>
        <li><strong>Mint:</strong> Create new tokens of the denom, credited to the admin itself</li>
        <li><strong>Burn:</strong> Destroy tokens from the admin&apos;s own balance</li>
        <li><strong>Change Admin:</strong> Transfer admin rights to another address</li>
        <li><strong>Update allow list / metadata / pause:</strong> Via <code>MsgUpdateDenom</code>, <code>MsgSetDenomMetadata</code>, and <code>MsgSetDenomPaused</code></li>
      </ul>

      <p>
        There is no force-transfer message in this module: the admin cannot move or burn tokens held by other accounts. Ordinary transfers use the bank module and require the holder&apos;s signature. Admins can share privileges via the authz module without changing the master admin.
      </p>

      <h2>Queries</h2>

      <pre><code>{`paxd query tokenfactory params
paxd query tokenfactory denom-authority-metadata <denom>
paxd query tokenfactory denom-paused <denom>
paxd query tokenfactory denoms-from-creator pax166vhptur29s3gw5qr6dm30s06gej6pr4zevkqk
paxd query tokenfactory denom-metadata <denom>
paxd query tokenfactory denom-allow-list <denom>
paxd query bank denom-metadata --denom factory/pax166vh.../ufoo`}</code></pre>

      <h2>Parameters</h2>

      <ul>
        <li><strong><code>denom_allowlist_max_size</code>:</strong> Maximum addresses in a denom allow list. Default <code>2000</code>; changeable by governance</li>
      </ul>

      <h2>Restrictions</h2>

      <p>
        To fit within the 128-byte Cosmos SDK denom limit:
      </p>

      <ul>
        <li><strong>Max subdenom length:</strong> 44 characters</li>
        <li><strong>Max creator address length:</strong> 75 characters (bech32 prefix ≤ 16 chars)</li>
      </ul>

      <p>
        Calculation:
      </p>

      <pre><code>{`len("factory") + 2*len("/") + len(creator_address) + len(subdenom) ≤ 128
7 + 2 + 75 + 44 = 128`}</code></pre>

      <h2>Use Cases</h2>

      <ul>
        <li><strong>Wrapped assets:</strong> Mint tokens representing off-chain assets</li>
        <li><strong>Synthetic tokens:</strong> Create tokens backed by other on-chain assets</li>
        <li><strong>Protocol-managed denoms:</strong> Modules create denoms for internal accounting</li>
        <li><strong>Governance tokens:</strong> DAOs issue native governance tokens</li>
      </ul>

      <h2>Integration with Bank Module</h2>

      <p>
        Tokenfactory denoms are native tokens, not ERC-20 contracts. They integrate with the bank module for:
      </p>

      <ul>
        <li>Balance queries</li>
        <li>Transfers via <code>MsgSend</code></li>
        <li>IBC transfers</li>
        <li>Distribution to stakers</li>
      </ul>

      <h2>Next Steps</h2>

      <ul>
        <li><Link href="/precompiles">Review current precompile addresses and behavior</Link></li>
        <li><Link href="/modules">Review other Paxeer modules</Link></li>
      </ul>

      <div className="prev-next">
        <Link href="/modules/store">
          <div className="prev-next-label">Previous</div>
          <div className="prev-next-title">Store Module</div>
        </Link>
        <Link href="/precompiles">
          <div className="prev-next-label">Next</div>
          <div className="prev-next-title">Precompiles</div>
        </Link>
      </div>
    </DocsLayout>
  )
}
