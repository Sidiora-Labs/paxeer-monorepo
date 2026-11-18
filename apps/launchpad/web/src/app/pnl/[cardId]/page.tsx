import type { Metadata } from 'next';
import { cache } from 'react';
import { notFound } from 'next/navigation';
import Link from 'next/link';
import PnlCardLanding from '@/widgets/pnl/pnl-card-landing';
import { PublicShareError, readPublicShare } from '@/widgets/pnl/share-transport';
import { checkedPublicShare } from '@/widgets/pnl/share-disclosure';

interface PageProps { params: Promise<{ cardId: string }> }
export const dynamic = 'force-dynamic';
export const revalidate = 0;

const readCard = cache(async (cardId: string) => checkedPublicShare(await readPublicShare(cardId)));

export async function generateMetadata({ params }: PageProps): Promise<Metadata> {
  const { cardId } = await params;
  try {
    const share = await readCard(cardId);
    return {
      title: 'PnL card — Sidiora',
      description: 'Trading results shared on Sidiora',
      robots: { index: false, follow: false },
      openGraph: { title: 'PnL card — Sidiora', images: [{ url: `/api/og/pnl/${share.id}`, width: 1200, height: 630 }] },
    };
  } catch {
    return { title: 'Card not found — Sidiora', robots: { index: false, follow: false } };
  }
}

export default async function PnlCardPage({ params }: PageProps) {
  const { cardId } = await params;
  try {
    const share = await readCard(cardId);
    return <PnlCardLanding share={share} />;
  } catch (error) {
    if (error instanceof PublicShareError && error.status === 404) notFound();
    if (error instanceof PublicShareError && error.status === 410 || error instanceof Error && error.message === 'Card withdrawn or expired') {
      return <main className="mx-auto max-w-content-narrow-max-width p-6 text-text"><h1>This card is gone</h1><p>It was removed or has expired</p><Link href="/profile">Go to your profile</Link></main>;
    }
    return <main className="mx-auto max-w-content-narrow-max-width p-6 text-text"><h1>Can&apos;t load this card right now</h1><p>Try again in a moment</p><a href={`/pnl/${encodeURIComponent(cardId)}`}>Try again</a><Link href="/profile">Go to your profile</Link></main>;
  }
}
