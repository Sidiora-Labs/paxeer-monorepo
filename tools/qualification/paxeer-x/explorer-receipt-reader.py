"""Refuse the retired agentd startup configuration and receipt producer.

node_endpoint, expected_protocol_version and sequencer_authority_source no
longer select an explorer receipt source; chain receipts are authoritative.
"""

import argparse


def main():
    parser = argparse.ArgumentParser(
        description='The daemon-backed explorer receipt evidence producer is retired.'
    )
    parser.parse_args()
    parser.error('daemon-backed receipt evidence production is retired; use chain receipts')


if __name__ == '__main__':
    main()
