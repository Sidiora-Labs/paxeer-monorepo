import argparse


def main():
    parser = argparse.ArgumentParser(
        description='The daemon-backed explorer receipt evidence producer is retired.'
    )
    parser.parse_args()
    parser.error('daemon-backed receipt evidence production is retired; use chain receipts')


if __name__ == '__main__':
    main()
