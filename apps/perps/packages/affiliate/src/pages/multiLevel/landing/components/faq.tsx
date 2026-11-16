import { useTranslation } from "@paxeer-network/i18n";
import surface from "../../../../surface.module.css";

export const Faq = () => {
  const { t } = useTranslation();
  return (
    <section
      className={`oui-affiliate-faq ${surface.reading}`}
      aria-labelledby="oui-affiliate-faq-title"
    >
      <h2
        id="oui-affiliate-faq-title"
        className="oui-text-4xl oui-font-semibold"
      >
        {t("affiliate.faq.title")}
      </h2>
      <div className="oui-faq-list oui-mt-8">
        {[1, 2, 3, 4, 5].map((number) => (
          <details
            key={number}
            className="oui-border-b oui-border-line oui-py-4"
          >
            <summary className="oui-cursor-pointer oui-text-xl oui-font-semibold oui-py-3">{`${number}. ${t(`affiliate.faq.question${number}`)}`}</summary>
            <p className="oui-pt-3 oui-text-content-muted oui-font-medium">
              {t(`affiliate.faq.answer${number}`)}
            </p>
          </details>
        ))}
      </div>
    </section>
  );
};
