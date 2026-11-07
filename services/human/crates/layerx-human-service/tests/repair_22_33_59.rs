use std::error::Error;
use std::fs;

use layerx_human_service::audit::AuditChain;
use layerx_human_service::notify::{
    ActivityEntryId, Channel, DeepLinks, DetailLevel, DispatchOutcome, Dispatcher, Event, EventId,
    JourneyId, Money, NotificationClass, Preferences,
};
use layerx_human_service::store::{PrincipalScope, PrincipalStore, Table};
use layerx_human_service::trace::TraceId;
use layerx_human_test_support::{
    directory, install_and_open, principal, retention_uniform, row_key, tenancy,
};

type TestResult = Result<(), Box<dyn Error>>;

fn money_event(id: &str) -> Result<Event, Box<dyn Error>> {
    Ok(Event::MoneyArrived {
        entry_id: ActivityEntryId::new(format!("act_{id}"))?,
        journey_id: JourneyId::new(format!("jrn_{id}"))?,
        money: Money::new(77, "LXP")?,
    })
}

fn choices() -> Preferences {
    let mut preferences = Preferences::default();
    preferences.set_detail(DetailLevel::Full);
    preferences.set_channel(Channel::Email, false);
    preferences.set_class(Channel::Push, NotificationClass::MoneyArrived, false);
    preferences.set_class(Channel::InApp, NotificationClass::ClaimReady, false);
    preferences
}

fn notification_rows(scope: &PrincipalScope<'_>) -> Vec<(String, u64, Vec<u8>)> {
    scope
        .keys(Table::Notifications)
        .into_iter()
        .filter(|key| {
            key.as_str().starts_with("notify_delivery_")
                || key.as_str().starts_with("notify_batch_")
        })
        .map(|key| {
            let row = scope.get(Table::Notifications, &key).expect("listed row");
            (
                key.as_str().to_owned(),
                row.written_at(),
                row.bytes().to_vec(),
            )
        })
        .collect()
}

#[test]
fn notification_read_state_survives_sweep_and_reopen_without_changing_evidence() -> TestResult {
    let root = directory("repair-22-33-59-read");
    let policy = retention_uniform(0);
    let map = tenancy(&[("alice", "tenant-a"), ("bob", "tenant-b")]);
    let (mut store, digest) = install_and_open(&root, &map, policy);
    let trace = TraceId::mint([59; 16]);
    let (read_id, unread_id, evidence, deliveries);
    {
        let mut scope = store.principal(&principal("alice"))?;
        let mut audit = AuditChain::open(&scope)?;
        let read = Dispatcher::dispatch(&mut scope, &mut audit, 10, &trace, &money_event("read")?)?;
        let unread =
            Dispatcher::dispatch(&mut scope, &mut audit, 11, &trace, &money_event("unread")?)?;
        read_id = read.notification_id().clone();
        unread_id = unread.notification_id().clone();
        assert_eq!(DeepLinks::inventory(&scope, 12)?.unread_count(), 2);
        evidence = audit.export(&scope)?;
        deliveries = notification_rows(&scope);
        assert!(DeepLinks::mark_read(&mut scope, 12, &read_id)?.read());
        assert!(DeepLinks::mark_read(&mut scope, 13, &read_id)?.read());
        assert_eq!(DeepLinks::inventory(&scope, 13)?.unread_count(), 1);
        let report = scope.expire(1_000)?;
        assert!(report.expired_rows > 0, "history was actually swept");
        assert!(report.pinned_evidence_retained > 0);
        assert_eq!(DeepLinks::inventory(&scope, 1_000)?.unread_count(), 1);
        assert_eq!(audit.export(&scope)?, evidence);
        assert_eq!(notification_rows(&scope), deliveries);
    }
    drop(store);
    for now in [2_000, u64::MAX] {
        let mut reopened = PrincipalStore::open(&root, policy, digest)?;
        {
            let mut scope = reopened.principal(&principal("alice"))?;
            scope.expire(now)?;
            let inventory = DeepLinks::inventory(&scope, now)?;
            assert_eq!(inventory.unread_count(), 1);
            assert!(inventory
                .notification(&read_id)
                .expect("read notification")
                .read());
            assert!(!inventory
                .notification(&unread_id)
                .expect("unread notification")
                .read());
            assert!(DeepLinks::mark_read(&mut scope, now, &read_id)?.read());
            assert_eq!(AuditChain::open(&scope)?.export(&scope)?, evidence);
            assert_eq!(notification_rows(&scope), deliveries);
        }
        let scope = reopened.principal(&principal("bob"))?;
        assert_eq!(DeepLinks::inventory(&scope, now)?.unread_count(), 0);
    }
    fs::remove_dir_all(root)?;
    Ok(())
}

#[test]
fn notification_preferences_survive_sweep_reopen_and_explicit_replacement() -> TestResult {
    let root = directory("repair-22-33-59-preferences");
    let policy = retention_uniform(0);
    let map = tenancy(&[("alice", "tenant-a"), ("bob", "tenant-b")]);
    let (mut store, digest) = install_and_open(&root, &map, policy);
    let trace = TraceId::mint([60; 16]);
    let preferences = choices();
    {
        let mut scope = store.principal(&principal("alice"))?;
        Dispatcher::update_preferences(&mut scope, 10, &preferences)?;
        let mut audit = AuditChain::open(&scope)?;
        let report =
            Dispatcher::dispatch(&mut scope, &mut audit, 11, &trace, &money_event("before")?)?;
        assert_eq!(report.deliveries().len(), 1);
        assert_eq!(report.deliveries()[0].channel(), Channel::InApp);
        assert!(report.deliveries()[0]
            .payload()
            .contains("\"amount\":\"77\""));
        assert!(scope.expire(1_000)?.expired_rows > 0);
        assert_eq!(Dispatcher::preferences(&scope)?, preferences);
    }
    drop(store);
    let mut reopened = PrincipalStore::open(&root, policy, digest)?;
    {
        let mut scope = reopened.principal(&principal("alice"))?;
        assert_eq!(Dispatcher::preferences(&scope)?, preferences);
        let mut audit = AuditChain::open(&scope)?;
        let report = Dispatcher::dispatch(
            &mut scope,
            &mut audit,
            2_000,
            &trace,
            &money_event("after")?,
        )?;
        assert_eq!(report.deliveries().len(), 1);
        assert_eq!(report.deliveries()[0].channel(), Channel::InApp);
        assert!(report.deliveries()[0]
            .payload()
            .contains("\"amount\":\"77\""));
        let claim = Dispatcher::dispatch(
            &mut scope,
            &mut audit,
            2_001,
            &trace,
            &Event::ClaimReady {
                journey_id: JourneyId::new("jrn_claim")?,
                money: Money::new(77, "LXP")?,
            },
        )?;
        assert_eq!(claim.deliveries().len(), 1);
        assert_eq!(claim.deliveries()[0].channel(), Channel::Push);
        let mut replacement = Preferences::default();
        replacement.set_detail(DetailLevel::Minimal);
        for channel in Channel::ALL {
            replacement.set_channel(channel, false);
        }
        Dispatcher::update_preferences(&mut scope, 2_002, &replacement)?;
        scope.expire(u64::MAX)?;
        assert_eq!(Dispatcher::preferences(&scope)?, replacement);
    }
    {
        let scope = reopened.principal(&principal("bob"))?;
        assert_eq!(Dispatcher::preferences(&scope)?, Preferences::default());
    }
    drop(reopened);
    let mut reopened = PrincipalStore::open(&root, policy, digest)?;
    {
        let mut scope = reopened.principal(&principal("alice"))?;
        let saved = Dispatcher::preferences(&scope)?;
        assert_eq!(saved.detail(), DetailLevel::Minimal);
        assert!(Channel::ALL
            .into_iter()
            .all(|channel| !saved.channel(channel).enabled()));
        let mut audit = AuditChain::open(&scope)?;
        let suppressed = Dispatcher::dispatch(
            &mut scope,
            &mut audit,
            3_000,
            &trace,
            &money_event("suppressed")?,
        )?;
        assert_eq!(suppressed.outcome(), DispatchOutcome::Suppressed);
        assert!(suppressed.deliveries().is_empty());
        let security = Dispatcher::dispatch(
            &mut scope,
            &mut audit,
            3_001,
            &trace,
            &Event::SecurityRecovery {
                event_id: EventId::new("evt_recovery")?,
            },
        )?;
        assert_eq!(security.deliveries().len(), 1);
        assert_eq!(security.deliveries()[0].channel(), Channel::InApp);
        assert!(security.deliveries()[0]
            .payload()
            .contains("notification.minimal"));
    }
    fs::remove_dir_all(root)?;
    Ok(())
}

#[test]
fn legacy_notification_state_migrates_on_dispatch_before_sweep_and_reopen() -> TestResult {
    let root = directory("repair-22-33-59-legacy");
    let policy = retention_uniform(0);
    let map = tenancy(&[("alice", "tenant-a")]);
    let (mut store, digest) = install_and_open(&root, &map, policy);
    let trace = TraceId::mint([61; 16]);
    let preferences = choices();
    let event = money_event("legacy")?;
    let (id, evidence);
    {
        let mut scope = store.principal(&principal("alice"))?;
        Dispatcher::update_preferences(&mut scope, 10, &preferences)?;
        let mut audit = AuditChain::open(&scope)?;
        id = Dispatcher::dispatch(&mut scope, &mut audit, 11, &trace, &event)?
            .notification_id()
            .clone();
        DeepLinks::mark_read(&mut scope, 12, &id)?;
        evidence = audit.export(&scope)?;
        // Keep the production codec's rows in the exact pre-repair layout.
        assert!(scope.remove(Table::EventOutbox, &row_key("notify_preferences"))?);
        assert!(scope.remove(
            Table::EventOutbox,
            &row_key(&format!("notify_read_{}", id.as_str()))
        )?);
    }
    drop(store);
    let mut reopened = PrincipalStore::open(&root, policy, digest)?;
    {
        let mut scope = reopened.principal(&principal("alice"))?;
        assert_eq!(Dispatcher::preferences(&scope)?, preferences);
        assert_eq!(DeepLinks::inventory(&scope, 13)?.unread_count(), 0);
        let mut audit = AuditChain::open(&scope)?;
        let replay = Dispatcher::dispatch(&mut scope, &mut audit, 13, &trace, &event)?;
        assert_eq!(replay.outcome(), DispatchOutcome::Deduplicated);
        Dispatcher::migrate_retained_state(&mut scope)?;
        assert!(scope.expire(1_000)?.expired_rows > 0);
        assert_eq!(audit.export(&scope)?, evidence);
    }
    drop(reopened);
    let mut reopened = PrincipalStore::open(&root, policy, digest)?;
    {
        let mut scope = reopened.principal(&principal("alice"))?;
        scope.expire(u64::MAX)?;
        assert_eq!(Dispatcher::preferences(&scope)?, preferences);
        let inventory = DeepLinks::inventory(&scope, u64::MAX)?;
        assert_eq!(inventory.unread_count(), 0);
        assert!(inventory
            .notification(&id)
            .expect("legacy notification")
            .read());
        assert_eq!(AuditChain::open(&scope)?.export(&scope)?, evidence);
    }
    fs::remove_dir_all(root)?;
    Ok(())
}

#[test]
fn legacy_migration_refuses_corruption_without_partially_saving_choices() -> TestResult {
    let root = directory("repair-22-33-59-corruption");
    let map = tenancy(&[("alice", "tenant-a")]);
    let (mut store, _) = install_and_open(&root, &map, retention_uniform(0));
    {
        let mut scope = store.principal(&principal("alice"))?;
        let preferences = choices();
        Dispatcher::update_preferences(&mut scope, 10, &preferences)?;
        let mut audit = AuditChain::open(&scope)?;
        let id = Dispatcher::dispatch(
            &mut scope,
            &mut audit,
            11,
            &TraceId::mint([62; 16]),
            &money_event("corrupt")?,
        )?
        .notification_id()
        .clone();
        let evidence = audit.export(&scope)?;
        assert!(scope.remove(Table::EventOutbox, &row_key("notify_preferences"))?);
        let read_key = row_key(&format!("notify_read_{}", id.as_str()));
        scope.put(
            Table::Notifications,
            read_key,
            12,
            b"invalid read marker".to_vec(),
        )?;
        assert!(Dispatcher::migrate_retained_state(&mut scope).is_err());
        assert!(scope
            .get(Table::EventOutbox, &row_key("notify_preferences"))
            .is_none());
        assert!(DeepLinks::inventory(&scope, 13).is_err());
        assert_eq!(audit.export(&scope)?, evidence);
        scope.put(
            Table::EventOutbox,
            row_key("notify_preferences"),
            13,
            b"invalid preferences".to_vec(),
        )?;
        assert!(Dispatcher::preferences(&scope).is_err());
    }
    drop(store);
    fs::remove_dir_all(root)?;
    Ok(())
}
