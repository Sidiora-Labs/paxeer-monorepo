#!/usr/bin/env python3
import json
from pathlib import Path
import re
import sys
import unittest

sys.dont_write_bytecode = True
ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / 'tools/paxeer-x'))
import candidate
from candidate import SERVICE_CATALOGUE


class ServiceCatalogueTest(unittest.TestCase):
    def test_catalogue_reads_layerx_merge_services(self):
        self.assertEqual(SERVICE_CATALOGUE, 'spec/layerx-merge/spec.kvx')
        path = ROOT / SERVICE_CATALOGUE
        services = candidate.catalogue(path)
        self.assertEqual(len(services), 40)
        blocks = re.findall(r'^\[service\.([a-z0-9-]+)\]\n(.*?)(?=^\[|\Z)',
                            path.read_text(encoding='utf-8'), flags=re.M | re.S)
        self.assertEqual(len(blocks), 40)
        self.assertEqual(set(services), {name for name, block in blocks})
        dependency_types = set()
        for name, block in blocks:
            with self.subTest(service=name):
                declaration = services[name]
                self.assertEqual(set(declaration), {'purpose', 'route', 'dependencies'})
                for field in ('purpose', 'route', 'dependencies'):
                    value = re.search(r'^' + field + r'\s*=\s*(.+)$', block, flags=re.M)
                    self.assertIsNotNone(value)
                    self.assertEqual(declaration[field], json.loads(value[1]))
                for field in ('purpose', 'route'):
                    self.assertIsInstance(declaration[field], str)
                    self.assertTrue(declaration[field])
                dependencies = declaration['dependencies']
                self.assertIsInstance(dependencies, (str, list))
                dependency_types.add(type(dependencies))
                if isinstance(dependencies, list):
                    for dependency in dependencies:
                        self.assertIsInstance(dependency, str)
                        self.assertTrue(dependency)
        self.assertEqual(dependency_types, {str, list})


if __name__ == '__main__':
    unittest.main()
