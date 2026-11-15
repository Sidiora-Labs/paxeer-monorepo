defmodule Explorer.Test.TLSCertificates do
  @moduledoc false

  def create! do
    directory = Path.join(System.tmp_dir!(), "explorer-tls-#{System.unique_integer([:positive])}")
    File.mkdir_p!(directory)
    ExUnit.Callbacks.on_exit(fn -> File.rm_rf!(directory) end)

    for authority <- ["trusted", "untrusted"] do
      openssl!(directory, [
        "req",
        "-x509",
        "-newkey",
        "rsa:2048",
        "-nodes",
        "-sha256",
        "-days",
        "2",
        "-subj",
        "/CN=Explorer #{authority} test CA",
        "-addext",
        "basicConstraints=critical,CA:TRUE",
        "-addext",
        "keyUsage=critical,keyCertSign,cRLSign",
        "-keyout",
        "#{authority}-ca-key.pem",
        "-out",
        "#{authority}-ca.pem"
      ])
    end

    for {name, authority, hostname} <- [
          {"trusted-server", "trusted", "localhost"},
          {"untrusted-server", "untrusted", "localhost"},
          {"wrong-host-server", "trusted", "other.invalid"}
        ] do
      openssl!(directory, [
        "req",
        "-new",
        "-newkey",
        "rsa:2048",
        "-nodes",
        "-sha256",
        "-subj",
        "/CN=#{hostname}",
        "-keyout",
        "#{name}-key.pem",
        "-out",
        "#{name}.csr"
      ])

      File.write!(Path.join(directory, "#{name}.ext"), """
      basicConstraints=critical,CA:FALSE
      keyUsage=critical,digitalSignature,keyEncipherment
      extendedKeyUsage=serverAuth
      subjectAltName=DNS:#{hostname}
      """)

      openssl!(directory, [
        "x509",
        "-req",
        "-sha256",
        "-days",
        "2",
        "-in",
        "#{name}.csr",
        "-CA",
        "#{authority}-ca.pem",
        "-CAkey",
        "#{authority}-ca-key.pem",
        "-CAcreateserial",
        "-extfile",
        "#{name}.ext",
        "-out",
        "#{name}.pem"
      ])
    end

    directory
  end

  defp openssl!(directory, arguments) do
    case System.cmd("openssl", arguments, cd: directory, stderr_to_stdout: true) do
      {_output, 0} -> :ok
      {output, status} -> raise "Certificate generation exited with #{status}: #{output}"
    end
  end
end
