# Launchpad visual brief

This brief states the look of the launchpad web application
(`apps/launchpad/web`). It is the reference for every view of the application:
where a view and this brief differ, the view changes. The brief names roles,
values and compositions; the token names are those of
`apps/launchpad/web/src/app/tokens.css`.

The look in one sentence: a dark, quiet trading feed in which the coin, its
value and its direction are the only loud things on the screen.

## 1. Principles

1. **Dark first.** The application renders in the dark set for every visitor
   who has not chosen otherwise. The light set stays complete and selectable
   (section 9). No view is designed in one set only.
2. **Content is the colour.** Surfaces are near-black neutrals with almost no
   steps between them. Colour comes from coin images, from the direction of a
   value (up, down) and from the one accent. A panel does not get a coloured
   fill, a coloured border or a shadow to look important.
3. **One accent, two directions.** The accent marks the primary action and a
   rising value. The danger colour marks selling and a falling value. No third
   colour competes with them; warning and info appear only in notices.
4. **Rows, not boxes.** Lists are flat rows separated by hairlines, each row
   one tap target. Cards are for things that stand alone: the trade panel, a
   callout, a position, a form.
5. **Numbers are the headline.** Market value, price and change are the
   largest and heaviest text of their row, right aligned, in tabular figures.
   Labels are small and muted.
6. **Round and soft.** Controls and cards have generous radii, coin images are
   rounded squares, floating layers are translucent with a blur.
7. **Every state is designed.** A view has a waiting form that reserves the
   size of its loaded form, an empty form that says what will appear, and a
   failure form with one way forward. A state of the product (a coin before its
   first trade) is never shown as a failure.
8. **Thumb first.** At phone widths the primary actions sit at the bottom
   within reach: the bar of destinations and, on a coin, the trade action.
   Touch controls measure at least 44 pixels up to 834 wide.
9. **Calm motion.** Motion confirms a change (a value that moves, a row that
   arrives) and lasts 120 to 260 ms. Under reduced motion nothing moves.
10. **The rules that already hold stay.** Focus marks, labels, roles and
    landmarks, the contrast rule (text 4.5:1, large text and the edges and
    fills that identify a control 3:1), the declared type steps and the
    reserved scroll bar track are not traded for looks. Where a value of this
    brief fails the contrast rule on its surface, the value moves in lightness
    until the rule holds and keeps its hue.
11. **Structure and sizes stay.** The columns, panels, tabs and routes of each
    view stay, and so do the sizes that are set today: the type steps, the
    heights of bars, rows and controls, the widths of columns. No size grows.
    This brief changes colour, surface, type weight, shape, the composition
    inside a panel and every state; section 7 names the few places where the
    arrangement of a view changes.

## 2. Colour tokens

Each role has a dark and a light value; components read the role, never a
literal colour.

| Role | Token | Dark | Light |
| --- | --- | --- | --- |
| Canvas | `--sl-canvas` | `#0b0b0e` | `#f7f9fc` |
| Surface 1 (cards, panels, sheets) | `--sl-surface-1` | `#121216` | `#ffffff` |
| Surface 2 (inputs, chips, inner cards) | `--sl-surface-2` | `#19191f` | `#eaf0f5` |
| Surface 3 (hairlines, tracks, waiting blocks) | `--sl-surface-3` | `#23232b` | `#dfe7ee` |
| Surface hover | `--sl-surface-hover` | `#2a2a33` | `#d3dee8` |
| Surface active | `--sl-surface-active` | `#34343f` | `#c7d6e2` |
| Text | `--sl-text` | `#f5f5f7` | `#101c29` |
| Text muted (secondary lines, labels) | `--sl-text-muted` | `#a6a6b0` | `#34495e` |
| Text faint (meta, units, placeholders) | `--sl-text-faint` | `#8e8e99` | `#435568` |
| Text disabled | `--sl-text-disabled` | `#5c5c66` | `#657587` |
| Control edge | `--sl-control-edge` | `#70707c` | `#657587` |
| Accent (primary action, up) | `--sl-accent` | `#44e8c8` | `#006852` |
| Accent hover / active | `--sl-accent-hover`, `--sl-accent-active` | `#6bedd5` / `#2bb89e` | `#005844` / `#004735` |
| On accent | `--sl-on-accent` | `#06201b` | `#ffffff` |
| Accent soft / soft strong | `--sl-accent-soft`, `--sl-accent-soft-strong` | `#11302b` / `#1a4a42` | `#d7f4e9` / `#b9e7d8` |
| Danger (sell, down) | `--sl-danger` | `#ff6f73` | `#ac1826` |
| Danger strong | `--sl-danger-strong` | `#ff8d90` | `#a11422` |
| On danger | `--sl-on-danger` | `#240708` | `#ffffff` |
| Danger soft | `--sl-danger-soft` | `#381a1d` | `#ffe5e7` |
| Warning / soft | `--sl-warning`, `--sl-warning-soft` | `#ffd24a` / `#2e2a18` | `#795400` / `#fff0c2` |
| Info / soft | `--sl-info`, `--sl-info-soft` | `#a6baff` / `#1f2438` | `#2d49a9` / `#e6ebff` |
| Inverse surface (the one light button on dark) | `--sl-surface-inverse` | `#f5f5f7` | `#101c29` |
| Focus | `--sl-focus` | `#8bffc5` | `#005a48` |

Rules:

- The dark neutrals carry no blue cast. The light column keeps the values the
  application holds today.
- A hairline is one pixel of surface 3. It separates rows and frames an input
  at rest; it does not frame a card that already differs from its ground.
- Direction: up reads `--sl-percent-up` on `--sl-percent-up-soft`, down reads
  `--sl-percent-down` on `--sl-percent-down-soft`, no change reads the muted
  text on surface 2. Candles, volume bars and trade sides use the same pair.
- Row wash: a row that shows a change carries a wash of its direction colour
  that starts transparent at the middle of the row and reaches 10 percent
  opacity at the right edge (`--sl-tint-accent-row-wash`,
  `--sl-tint-danger-row-wash`). A row without a change carries none.
- Floating layers (sheets, menus, toasts) use surface 2 at 82 percent opacity
  over a 16 pixel backdrop blur with a hairline; where blur is not supported
  the layer is the opaque surface 2. The bar of destinations, the sticky trade
  bar, the cookie notice and the action bar of a create view are opaque, the
  bar of destinations in surface 1 and the others in surface 2, each with its
  hairline.
- Brand colours of wallets appear only on their own marks.

## 3. Type

The family stays the one the application loads today, with the system sans as
fallback. Weights become lighter: text is read, numbers are stressed.

| Step | Size / line | Weight | Use |
| --- | --- | --- | --- |
| 2xs | 11 / 14 | 600 | badges, column heads (upper case, wide tracking) |
| xs | 12 / 16 | 500 | meta lines, helper text, units |
| sm | 13 / 18 | 500 | secondary lines, table cells |
| base | 14 / 20 | 500 | body, controls, names in dense tables |
| md | 16 / 24 | 600 | names in rows, card titles, inputs |
| lg | 20 / 28 | 700 | view titles, values in a row at tablet width and above |
| xl | 24 / 32 | 700 | the value of a coin header, panel totals |
| 2xl | 32 / 40 | 700 | the amount of the trade panel, balances |
| display | 48 / 52 | 700 | one figure per view at most |
| num-sm, num-md | 13 / 18, 14 / 20 | 600 | figures in tables and chips |

Rules:

- Every figure (price, value, change, count, time) is set in tabular figures.
- A view has one title of step lg; panels and sections title in md. No view
  stacks two titles above its content.
- Weight 800 is not used. A name is 600, a value 700, a label 500.
- Text never wraps inside a control. A name that does not fit ends in an
  ellipsis and gives its full form to assistive technology.

## 4. Shape, space and motion

- Radii: chips and badges 8, controls (buttons, inputs, segments) 12, cards
  and panels 16, sheets 20 at the top corners, pills fully round. A coin image
  is a rounded square with a radius of 22 percent of its side; an account
  image is a circle.
- Control heights stay 28 (inline, inside a dense table), 36 (default at
  tablet width and above) and 44 (default at phone and tablet widths, and the
  trade and creation actions at every width).
- Space runs on the 4 pixel grid. Inside a card 16; between cards 12 at phone
  widths and 16 above; between the lines of a row 2; between the image of a
  row and its text 12.
- Rows keep the heights they have today.
- Elevation: none on the page plane. Only floating layers cast the pop shadow.
- Motion: 120 ms for hover and press, 180 ms for a tab, a segment or a row
  that arrives, 260 ms for a sheet. A value that changes tints in its
  direction colour for 600 ms and returns. A new row at the head of a live
  list slides in from above once; the list does not jump under a pointer or a
  finger that rests on it.

## 5. Components

Each component keeps the size it has today unless a line below names one.

**Buttons.** Primary: accent fill, on-accent text, weight 700. Secondary:
surface 2 with a hairline, text colour. Quiet: no fill until hover. Sell and
destructive: danger fill, on-danger text. Inverse: the inverse surface with
the canvas colour as text; it is used for one thing only, the action that
brings a visitor in (sign in, connect, add funds). One primary button per
view region.

**Direction chip.** Radius 6, an arrow (up or down) and the percentage in
num-sm; soft fill and text of the direction. It sits under or beside the value
it qualifies, never alone.

**Badges.** Outline pill in 2xs upper case: hot (danger outline and text), new
(info), graduated (accent), creator, pool. A verified mark is an accent disc
with a tick on the lower right corner of the image.

**Tabs.** A view switches its lists with text tabs: muted text, the current
tab in text colour with a 2 pixel underline of the text colour, on a hairline
that spans the view. Inside a panel a choice between few values (time frame,
price or value, list or grid) is a segmented control: surface 2 track, the
chosen segment a surface 3 pill in text colour. Tabs scroll sideways at
phone widths; they do not wrap.

**Coin row.** From the left: the image; the name in weight 600 with its
badges, and under it the ticker and the age in muted text. From the right: the
market value in weight 700 and under or beside it the direction chip. The
whole row is one link with the wash of section 2; hover raises it to surface
1. A row always shows the image of its coin or the initials form of the image
on surface 3.

**Coin card.** For grids: the image as a rounded square at the left, name,
ticker, creator and age, two lines of description at most, market value with
its direction chip and, for a coin on its curve, the progress line.

**Progress line.** A 6 pixel track of surface 3 with an accent fill, fully
round, with its label at the left and its percentage at the right in xs.

**Callout card.** The image of the author, name and time; the text; an inner
card (surface 2, radius 12) with the coin, the position and its result; one
row of quiet actions with their counts. Cards of a feed are separated by
hairlines, not by gaps.

**Inputs.** Surface 2 fill with a hairline, radius 12, label above in sm
weight 600, helper under it in xs muted. Focus shows the focus ring and keeps
the hairline. A failure shows the danger edge and one line of text; the field
is not painted red.

**Amount field.** The figure in the largest step the panel has today, the unit
as a chip at the right, the balance under it in xs with a control for the
maximum. Preset chips (secondary) under the field. Opening the field at phone
widths brings the numeric keyboard.

**Option row.** A group of a form that has a default (links, pair, rewards,
rules) is one closed row: its name at the left, its present choice in muted
text and a chevron at the right. It opens in place and closes again; a group
with a failure opens by itself.

**Tables.** No outer frame. Column heads in 2xs muted on the canvas; rows
separated by hairlines; figures right aligned; one sorted column marked by its
arrow.

**Panels.** A panel is surface 1 with radius 16 and no border on the canvas.
Panels that stand in one column are separated by the gap of section 4, and a
panel inside a panel is surface 2 with radius 12. A column that reaches the
edge of the view (a board column, a rail) is flush and separated by a
hairline instead.

**States.** Waiting: blocks of surface 3 in the shape and size of the loaded
content, with one slow shine. Empty: a muted icon of 24, a title in md, one
muted line, at most one secondary action; the form is as high as its content
and does not stretch a panel. Failure: the same form with a warning icon, the
reason in plain words and a retry. A notice inside a panel is one line with an
icon, not a second panel. A first state of the product (a coin before its
first trade, a ranking before its first period, a list before its first
entry) uses the empty form with neutral words and never the failure form, a
warning mark or a retry.

**Toasts and sheets.** A toast is a floating layer of at most 356 wide above
the fixed controls at the bottom. A sheet rises from the bottom at phone
widths with a grab mark and covers at most 85 percent of the height; at tablet
width and above the same content is a centred dialog or a popover.

## 6. Shell

The shell keeps its parts, their places and their heights.

- **Top bar.** Canvas colour with a hairline under it. The search field is a
  pill of surface 2 with the shortcut hint. To the right the primary button to
  create a coin and the account: signed out one inverse button to sign in,
  signed in the balance and the account image. The account menu holds the
  colour mode control.
- **Rail.** Canvas colour with a hairline at its right. An item is an icon
  with its label in weight 500; the current item is a surface 2 pill in text
  colour. The accent fills the create action alone, never a destination. The
  lower group holds the colour mode control.
- **Activity strip.** One line under the top bar on the canvas, its filters as
  quiet icon buttons, its entries as chips (account, side in its direction
  colour, amount, coin). With nothing to show it holds one muted line in xs
  and no stressed call to action.
- **Bar of destinations (phone widths).** Opaque surface 1 with a hairline
  above it. An item is an icon with its label in 2xs; the current item is in
  text colour, the others are muted; the create action is the accent disc.
  Content ends above the bar.
- **Footer.** One muted line in xs at tablet width and above: the legal links
  at the left, the figures of the platform at the right. At phone widths two
  lines at the end of the content.
- **Cookie notice.** A floating card of at most 360 wide at the lower left at
  tablet width and above. At phone widths one card above the bar of
  destinations, at most 96 high, with the text in two lines at most and both
  buttons side by side. It never covers a trade action or the bar of
  destinations.

## 7. Views

Every view takes the tokens, the components, the panels and the states of the
sections above. The arrangement of a view stays, except where a paragraph
below names a change.

**Home.** From 1024 the board of columns stays: flush columns separated by
hairlines, each with its head (title, tabs or segments, the live mark) on the
canvas and its rows in the form of the coin row or the callout card. Below
1024 the home gets a third tab and a new order: coins, callouts, top, with
coins first. The coins tab shows the ranking of the first board column as coin
rows, so that the first screen of a phone shows coins whenever the platform
has one.

**Explore, new, search, watchlist.** The presets are text tabs or chips of one
kind, not both; filters, columns and the switch between list and grid are
quiet icon buttons at the right. The table is the table of section 5 with the
coin row as its first column. The empty form of a list stands inside the table
frame, under the heads.

**Coin.** The columns stay.

- Header: at phone widths two rows and no more: image, name and ticker with
  the icon buttons; then the value with its direction chip and the stats in
  one line that scrolls sideways. Nothing in it wraps to a third row. The
  address and the secondary controls move behind the menu at phone widths.
- Chart: no frame, canvas ground, quiet grid, candles and volume in the
  direction colours. One toolbar row: the time frames as a segmented control,
  price or market value, and a menu for the rest (trade markers, currency).
  The row scrolls sideways at phone widths and never wraps.
- Before the first trade the chart area shows the empty form of the
  application (no trades yet, the chart starts with the first trade, the
  opening price) and the header shows a neutral chip. The drawing of the chart
  library for missing data and its warning marks never show. A line that
  offers a retry appears only after a read has failed.
- Trade panel: buy and sell as two segments (the chosen side filled in its
  colour), the amount field, the presets, the summary lines, the settings
  behind one icon, and the action: accent to buy, danger to sell, inverse to
  sign in or connect.
- Side column: trade panel, holders, stats and pool information are panels of
  section 5 with one title each; a label and its value share a line, values
  right aligned in tabular figures.
- Phone widths: the trade action stays in view above the bar of destinations
  and opens the trade sheet. The order of the page is header, value, chart,
  position, tabs.
- Trades: account, side, amounts, relative time, link to the transaction; a
  new row arrives with one wash. Holders: rank, account with its badges, share
  with a thin bar.

**Create.** The form and its preview card stay. The order: image (a square
drop area with the rules in one line), name, ticker, description, then the
groups that have a default (social links, pair, creator rewards and their
receiver, trading rules) each as one closed option row. The cost summary and
the action end the form and stay in view at phone widths.

**Profile and positions.** A head with the account image, name, the total
value with its result and period; counts in one line; tabs; positions as coin
rows whose right side shows the value of the position and its result. The
signed-out form is the empty form with the inverse button.

**Ranking and rewards.** Tables of section 5 with the first three places
marked by their rank colours; totals as figures above the table, not in tinted
boxes. A ranking before its first period shows one empty form, not a tinted
banner with several lines.

**Live, terminal, swap, orders, chat, bounties, documents.** They take the
tokens, the components and the shell of this brief without a change of their
arrangement.

## 8. Widths

The breakpoints and the layout of each width stay as they are today: one
column with the bar of destinations and sheets below 768, the collapsed rail
from 768, the columns of a view from 1024 and 1280.

No view scrolls sideways as a whole at any width from 360. A list or a row of
controls that is wider than its place scrolls inside its own frame and does
not wrap into further rows.

## 9. Colour mode

The document carries the chosen mode as an attribute on its root element,
written before the first paint: dark without a choice, light or dark by choice,
or the mode of the device where the visitor chooses that. The choice lives in
the account menu and in the lower group of the rail, is kept on the device and
applies to the chart as well. Both sets pass the contrast rule on every
surface they are used on.
