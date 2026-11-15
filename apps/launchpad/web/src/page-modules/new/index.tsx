import { redirect } from 'next/navigation';

export const dynamic = 'force-static';

export default function NewModule() {
  redirect('/explore?preset=new');
}
