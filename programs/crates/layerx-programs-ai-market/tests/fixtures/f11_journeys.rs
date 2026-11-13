//! Committed native producers, derived from the landed F11 integration fixtures.
//! Each observation is rerouted natively and measured in the production ABI-5 guest.
use super::f11_observe as observe;
#[allow(dead_code)]
#[path = "f11_bonds.rs"]
mod bonds;
#[allow(dead_code)]
#[path = "f11_canary_challenge.rs"]
mod canary;
#[allow(dead_code)]
#[path = "f11_payout_split.rs"]
mod payout;
#[allow(dead_code)]
#[path = "f11_placement_book.rs"]
mod placement;
#[path = "f11_producers.rs"]
mod producers;
#[path = "f11_rooms_and_deals.rs"]
mod rooms;
#[path = "f11_views.rs"]
mod views;
pub fn run() {
    rooms::recorded_journey();
    bonds::recorded_journey();
    views::recorded_journey();
    canary::recorded_journey();
    payout::recorded_journey();
    placement::recorded_journey();
}

pub fn room_profile_wire_round_trip() {
    rooms::room_profile_wire_round_trip();
}
