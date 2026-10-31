#!/usr/bin/env python3
import argparse


def main():
    parser = argparse.ArgumentParser(
        description='The standalone budget proof intake qualification gate is retired.'
    )
    parser.parse_args()
    parser.exit(
        1,
        'budget proof intake refused: standalone transport qualification is retired\n',
    )


if __name__ == '__main__':
    main()
