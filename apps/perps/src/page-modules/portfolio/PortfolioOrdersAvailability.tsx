import React from "react";
import { SectionCard } from "@/widgets/portfolio/PortfolioPrimitives";

export function PortfolioOrdersAvailability() {
  return (
    <SectionCard title="Open orders">
      <p className="text-sm text-muted-foreground">
        Native perps order management is not available in Portfolio yet.
        View and cancel your orders on the trading page.
      </p>
      <a href="/perps" className="inline-block mt-3 text-sm underline">
        Open trading page
      </a>
    </SectionCard>
  );
}
