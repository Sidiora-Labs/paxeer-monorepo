defmodule Explorer.Repo.Migrations.AddPaxeerXPriceObservationAccepted do
  use Ecto.Migration

  def change do
    alter table(:paxeer_x_price_observations) do
      add(:accepted, :boolean, null: false, default: true)
    end
  end
end
