"""Offline XML and plan discrimination for both independent vector workers."""
import copy
import importlib.util
from pathlib import Path
import sys
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]


def load(path, name):
    spec = importlib.util.spec_from_file_location(name, path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


class VectorContracts(unittest.TestCase):
    def test_both_workers_reject_active_external_and_wrong_size_svg(self):
        for role in ['inkscape-illustrator', 'inkscape-controlled-illustrator']:
            module = load(ROOT/'workers'/role/'expert/tools/svg_contract.py', role)
            with tempfile.TemporaryDirectory() as tmp:
                path = Path(tmp)/'drawing.svg'
                good = '<svg xmlns="http://www.w3.org/2000/svg" width="256" height="256" viewBox="0 0 256 256"><title>Fixture</title><desc>Synthetic shapes</desc><rect width="256" height="256" fill="#224466"/></svg>'
                path.write_text(good)
                module.inspect_svg(path, 256, 256)
                for bad in [good.replace('</svg>', '<script>alert(1)</script></svg>'),
                            good.replace('</svg>', '<image href="https://example.invalid/a.png"/></svg>'),
                            good.replace('<rect ', '<rect onclick="alert(1)" '),
                            good.replace('width="256"', 'width="257"', 1), '<svg>']:
                    with self.subTest(role=role, bad=bad):
                        path.write_text(bad)
                        with self.assertRaises((module.ContractError, ValueError)):
                            module.inspect_svg(path, 256, 256)

    def test_controlled_plan_rejects_unsafe_and_out_of_bounds_operations(self):
        folder = ROOT/'workers/inkscape-controlled-illustrator/expert/tools'
        sys.path.insert(0, str(folder))
        self.addCleanup(lambda: sys.path.remove(str(folder)))
        module = load(folder/'authoring.py', 'controlled_authoring')
        plan = dict(schema='inkscape-plan/v1', title='Synthetic curve', description='A bounded field',
                    layers=[dict(id='field', name='Field', objects=[dict(op='rect', id='back', fill='#224466',
                        stroke='none', stroke_width=0, opacity=1, x=0, y=0, width=256, height=256)])],
                    review={'targets': [{'id': 'back', 'importance': 'primary'}]})
        plan['layers'].append(dict(id='accent', name='Accent', objects=[dict(op='rect', id='mark', fill='#ddeeff', stroke='none', stroke_width=0, opacity=1, x=10, y=10, width=20, height=20)]))
        module.dom(plan, 256, 256)
        for mutate in [lambda p: p.update(title='<svg/>'),
                       lambda p: p['layers'][0]['objects'][0].update(width=257),
                       lambda p: p['layers'][0]['objects'][0].update(op='script'),
                       lambda p: p['layers'][0]['objects'][0].update(onload='code')]:
            value = copy.deepcopy(plan)
            mutate(value)
            with self.subTest(plan=value), self.assertRaises(ValueError):
                module.dom(value, 256, 256)


if __name__ == '__main__':
    unittest.main()
