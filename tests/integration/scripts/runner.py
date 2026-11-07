import argparse
import ast
import math
import os
import re
import signal
import subprocess

import yaml


class CommandExecutionError(RuntimeError):
    def __init__(self, returncode, stdout, stderr, timed_out=False):
        self.returncode = returncode
        self.stdout = stdout
        self.stderr = stderr
        self.timed_out = timed_out
        reason = (
            "deadline exceeded" if timed_out else f"unexpected exit status {returncode}"
        )
        super().__init__(f"Integration command failed: {reason}")


class TestRunner:

    def __init__(self, timeout=300, docker=True):
        self._validate_timeout(timeout)
        self.timeout = timeout
        self.docker = docker

    @staticmethod
    def _validate_timeout(timeout):
        if (
            isinstance(timeout, bool)
            or not isinstance(timeout, (int, float))
            or not math.isfinite(timeout)
            or timeout <= 0
        ):
            raise ValueError("Command timeout must be a positive finite number")

    def load_yaml_file(self, filepath):
        with open(filepath, 'r') as f:
            data = yaml.safe_load(f)
        return data

    # Function to process YAML
    def process_data(self, data):
        for test in data:
            self.run_test(test)

    # Function to execute a single test case
    def run_test(self, test):
        test_name = test["name"]
        print("\n========== " + test_name + " =========", flush=True)
        inputs = test["inputs"]
        env_map = {}
        for input in inputs:
            cmd = input['cmd']
            container = input.get("node", "pax-node-0")
            print(f'Input : {cmd}', flush=True)
            output = self.run_bash_command(
                cmd,
                self.docker,
                container,
                env_map,
                False,
                timeout=input.get('timeout', self.timeout),
                expected_status=input.get('expected_status', 0),
            )
            if input.get('env'):
                env_map[input['env']] = output
            result = output
            print(f'Output: {result}', flush=True)
        for verifier in test["verifiers"]:
            if not self.verify_result(env_map, verifier):
                print("Test failed for {}".format(verifier), flush=True)
                exit(1)
        print("Test Passed", flush=True)

    # Function to verify the result of a single test case
    def verify_result(self, env_map, verifier):
        type = verifier["type"]
        if type == "eval":
            expression = ast.parse(verifier["expr"].strip(), mode="eval")
            allowed = (
                ast.Expression, ast.Constant, ast.Name, ast.Load,
                ast.Compare, ast.Eq, ast.NotEq, ast.Lt, ast.LtE, ast.Gt, ast.GtE,
                ast.BoolOp, ast.And, ast.Or, ast.UnaryOp, ast.Not, ast.USub,
                ast.UAdd, ast.BinOp, ast.Add, ast.Sub, ast.Mult, ast.Div,
                ast.FloorDiv, ast.Mod,
            )
            if any(not isinstance(node, allowed) for node in ast.walk(expression)):
                raise ValueError("Unsupported integration verifier expression")
            values = {}
            for key, value in env_map.items():
                value = str(value)
                if re.fullmatch(r"[+-]?[0-9]+", value):
                    values[key] = int(value)
                else:
                    try:
                        number = float(value)
                    except ValueError:
                        values[key] = value
                    else:
                        values[key] = number if math.isfinite(number) else value
            return eval(
                compile(expression, "<integration verifier>", "eval"),
                {"__builtins__": {}},
                values,
            )
        elif type == "regex":
            env_key = verifier["result"]
            expression = str(verifier["expr"])
            result = env_map[env_key].strip()
            return re.match(expression, result)
        else:
            return False

    @staticmethod
    def _command_argv(command, docker, container, env_map):
        if isinstance(command, str):
            script = command
            if docker:
                script = 'export PATH=$PATH:/root/go/bin:/root/.foundry/bin && ' + script
            argv = ["/bin/bash", "-c", script]
        else:
            argv = list(command)
        if docker:
            docker_argv = ["docker", "exec"]
            for key, value in (env_map or {}).items():
                if not re.fullmatch(r"[A-Za-z_][A-Za-z0-9_]*", key):
                    raise ValueError("Invalid captured environment variable name")
                docker_argv.extend(["-e", f"{key}={value}"])
            argv = docker_argv + ["--", container] + argv
        return argv

    # Helper function to execute a single command within the docker container
    def run_bash_command(
        self, command, docker=False, container="", env_map=None,
        verbose=False, timeout=None, expected_status=0,
    ):
        timeout = self.timeout if timeout is None else timeout
        self._validate_timeout(timeout)
        if type(expected_status) is not int or not 0 <= expected_status <= 255:
            raise ValueError("Expected status must be an integer between 0 and 255")
        argv = self._command_argv(command, docker, container, env_map)
        environment = None
        if not docker and env_map:
            environment = os.environ.copy()
            environment.update(env_map)
        if verbose:
            print('Executing integration command', flush=True)
        process = subprocess.Popen(
            argv, stdout=subprocess.PIPE, stderr=subprocess.PIPE, shell=False,
            env=environment, start_new_session=True,
        )
        try:
            output, error = process.communicate(timeout=timeout)
        except subprocess.TimeoutExpired:
            try:
                os.killpg(process.pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
            output, error = process.communicate()
            raise CommandExecutionError(
                process.returncode, output.decode(errors="replace"),
                error.decode(errors="replace"), timed_out=True,
            ) from None
        if process.returncode != expected_status:
            raise CommandExecutionError(
                process.returncode, output.decode(errors="replace"),
                error.decode(errors="replace"),
            )
        return output.decode().strip()


def main():
    parser = argparse.ArgumentParser(
        description='Integration test runner',
        formatter_class=argparse.ArgumentDefaultsHelpFormatter,
    )
    parser.add_argument('filepath', type=str, help='The path to the JSON file')
    args = parser.parse_args()
    runner = TestRunner()
    data = runner.load_yaml_file(args.filepath)
    runner.process_data(data)


if __name__ == '__main__':
    main()
