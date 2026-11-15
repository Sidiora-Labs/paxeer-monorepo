import type { Metadata } from 'next';
import SearchModule from '@/page-modules/search';

export const metadata: Metadata = { title: 'Search coins and users' };

export default function SearchPage() {
  return <SearchModule />;
}
