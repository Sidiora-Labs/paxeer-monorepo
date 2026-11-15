'use client';

import { SvgIcon } from '@/components/ui/SvgIcon';
import { Dialog } from '@/components/ui/primitives';
import type { Contact } from '@/lib/contacts';
import { useLocale } from '@/providers/LocaleProvider';

interface ContactFormViewProps {
  view: 'add' | 'edit';
  editingContact: Contact | null;
  formName: string;
  setFormName: (v: string) => void;
  formAddress: string;
  setFormAddress: (v: string) => void;
  formNote: string;
  setFormNote: (v: string) => void;
  formError: string;
  deleteConfirmId: string | null;
  setDeleteConfirmId: (v: string | null) => void;
  onSave: () => void;
  onBack: () => void;
  onDelete: (id: string) => void;
}

export function ContactFormView({
  view,
  editingContact,
  formName,
  setFormName,
  formAddress,
  setFormAddress,
  formNote,
  setFormNote,
  formError,
  deleteConfirmId,
  setDeleteConfirmId,
  onSave,
  onBack,
  onDelete,
}: ContactFormViewProps) {
  const { t } = useLocale();

  return (
    <div className="min-h-screen flex flex-col px-4 pt-4 safe-area-pt">
      <div className="flex items-center gap-3 mb-6">
        <button
          type="button"
          onClick={onBack}
          aria-label="Back"
          className="min-h-[var(--touch-target)] p-2 -ml-2 press-scale"
        >
          <SvgIcon name="arrow-left" className="w-5 h-5" />
        </button>
        <h2 className="text-lg font-bold">
          {view === 'add' ? t.contacts.newContact : t.contacts.editContact}
        </h2>
        {view === 'edit' && editingContact && (
          <button
            type="button"
            onClick={() => setDeleteConfirmId(editingContact.id)}
            className="ml-auto p-2 press-scale"
            aria-label={t.contacts.deleteContact}
          >
            <SvgIcon name="trash" className="w-4.5 h-4.5" />
          </button>
        )}
      </div>

      <div className="flex-1 flex flex-col gap-4">
        <div>
          <label htmlFor="contact-name" className="text-xs text-pax-muted mb-1.5 block">
            {t.contacts.name}
          </label>
          <input
            id="contact-name"
            type="text"
            name="name"
            autoComplete="name"
            aria-invalid={Boolean(formError)}
            value={formName}
            onChange={(e) => setFormName(e.target.value)}
            placeholder={t.contacts.namePlaceholder}
            maxLength={50}
            className="w-full px-4 py-3 rounded-xl bg-pax-control   text-sm outline-none  transition-colors placeholder:text-pax-muted"
          />
        </div>

        <div>
          <label htmlFor="contact-address" className="text-xs text-pax-muted mb-1.5 block">
            {t.contacts.walletAddress}
          </label>
          <input
            id="contact-address"
            type="text"
            name="address"
            autoComplete="off"
            aria-invalid={Boolean(formError)}
            value={formAddress}
            onChange={(e) => setFormAddress(e.target.value)}
            placeholder="0x..."
            className="w-full px-4 py-3 rounded-xl bg-pax-control   text-sm font-mono outline-none  transition-colors placeholder:text-pax-muted"
          />
        </div>

        <div>
          <label htmlFor="contact-note" className="text-xs text-pax-muted mb-1.5 block">
            {t.contacts.note} <span className="text-pax-muted">({t.contacts.optional})</span>
          </label>
          <input
            id="contact-note"
            type="text"
            name="note"
            autoComplete="off"
            value={formNote}
            onChange={(e) => setFormNote(e.target.value)}
            placeholder={t.contacts.notePlaceholder}
            maxLength={100}
            className="w-full px-4 py-3 rounded-xl bg-pax-control   text-sm outline-none  transition-colors placeholder:text-pax-muted"
          />
        </div>

        {formError && (
          <div className="flex items-center gap-2" role="alert" aria-live="polite">
            <SvgIcon name="warning" className="w-3.5 h-3.5" />
            <p className="text-xs text-pax-error">{formError}</p>
          </div>
        )}
      </div>

      <div className="py-4 pb-24">
        <button
          type="button"
          onClick={onSave}
          className="min-h-[var(--touch-target)] w-full flex items-center justify-center gap-2 py-3 rounded-2xl bg-pax-accent text-pax-on-accent font-semibold text-sm press-scale transition-all"
        >
          <SvgIcon name="check" className="w-4 h-4" />
          {view === 'add' ? t.contacts.saveContact : t.contacts.updateContact}
        </button>
      </div>

      {deleteConfirmId && (
        <Dialog
          open={Boolean(deleteConfirmId)}
          onClose={() => setDeleteConfirmId(null)}
          title={`${t.contacts.deleteContact}?`}
        >
          <div className="space-y-4">
            <p className="text-xs text-pax-muted text-center">{t.contacts.deleteConfirm}</p>
            <div className="flex gap-3">
              <button
                type="button"
                onClick={() => setDeleteConfirmId(null)}
                className="min-h-[var(--touch-target)] flex-1 py-2.5 rounded-xl bg-pax-control text-sm font-medium press-scale focus-visible:ring-2 focus-visible:ring-pax-accent"
              >
                {t.common.cancel}
              </button>
              <button
                type="button"
                onClick={() => onDelete(deleteConfirmId)}
                className="min-h-[var(--touch-target)] flex-1 py-2.5 rounded-xl bg-pax-error-muted text-pax-error text-sm font-medium press-scale focus-visible:ring-2 focus-visible:ring-pax-error"
              >
                {t.common.confirm}
              </button>
            </div>
          </div>
        </Dialog>
      )}
    </div>
  );
}
