// This program updates the GCE machine definitions in //skolo/ansible/ssh.cfg based on machine
// descriptions returned by the "gcloud" command. It assumes that the gcloud command is installed
// and that the user has the necessary credentials.
package main

import (
	"bufio"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"

	"go.skia.org/infra/go/executil"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
)

const autogeneratedBlockBegin = "# BEGIN GCE MACHINES. DO NOT EDIT! This block is automatically generated."
const autogeneratedBlockEnd = "# END GCE MACHINES."

func main() {
	ctx := context.Background()
	sshCfgFile, err := getSshCfgFileFromArgs()
	ifErrThenDie(err)
	err = updateSshCfg(ctx, sshCfgFile)
	ifErrThenDie(err)
}

func getSshCfgFileFromArgs() (string, error) {
	if len(os.Args) != 2 {
		return "", skerr.Fmt("Usage: %s <path to ssh.cfg>", os.Args[0])
	}
	return os.Args[1], nil
}

func updateSshCfg(ctx context.Context, sshCfgFile string) error {
	// Read the existing ssh.cfg file into an array of lines.
	var sshCfgFileLines []string
	err := util.WithReadFile(sshCfgFile, func(f io.Reader) error {
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			sshCfgFileLines = append(sshCfgFileLines, scanner.Text())
		}
		sshCfgFileLines = append(sshCfgFileLines, "") // scanner.Scan deletes the last \n.
		return nil
	})
	if err != nil {
		return skerr.Wrap(err)
	}

	// Extract out the lines before and after the autogenerated block.
	linesBefore, linesAfter := parseLinesBeforeAndAfterAutogeneratedBlock(sshCfgFileLines)

	// Retrieve machine descriptions from GCE (name, external IP, OS).
	machines, err := describeGCEMachines(ctx)
	if err != nil {
		return skerr.Wrap(err)
	}

	// Filter out Windows machines for now.
	//
	// TODO(lovisolo): Find out if we can SSH into GCE Windows machines, and find an elegant way
	//                 to generate per-OS host groups in hosts.yml.
	var linuxMachines []machine
	for _, machine := range machines {
		if machine.os == "linux" {
			linuxMachines = append(linuxMachines, machine)
		}
	}

	// Generate host definitions based on the machine descriptions.
	autogeneratedBlock := generateHostDefinitionsBlock(linuxMachines)

	// Update the ssh.cfg file.
	allLines := append(linesBefore, autogeneratedBlock)
	allLines = append(allLines, linesAfter...)
	newSshCfgFileContents := strings.Join(allLines, "\n")
	err = util.WithWriteFile(sshCfgFile, func(w io.Writer) error {
		_, err := w.Write([]byte(newSshCfgFileContents))
		return skerr.Wrap(err)
	})
	return skerr.Wrap(err)
}

// parseLinesBeforeAndAfterAutogeneratedBlock reads an existing ssh.cfg file and returns the lines
// before and after the automatically generated block with GCE host definitions.
func parseLinesBeforeAndAfterAutogeneratedBlock(sshCfgFileLines []string) ([]string, []string) {
	var linesBefore []string
	var linesAfter []string

	type Location int
	const (
		BeforeBlock Location = iota
		InBlock
		AfterBlock
	)

	location := BeforeBlock
	for _, line := range sshCfgFileLines {
		switch location {
		case BeforeBlock:
			if strings.Contains(line, autogeneratedBlockBegin) {
				location = InBlock
			} else {
				linesBefore = append(linesBefore, line)
			}
		case InBlock:
			if strings.Contains(line, autogeneratedBlockEnd) {
				location = AfterBlock
			}
		case AfterBlock:
			linesAfter = append(linesAfter, line)
		default:
			panic("Unknown location") // Should never happen.
		}
	}

	return linesBefore, linesAfter
}

type machine struct {
	name       string
	externalIP string
	os         string // Either "linux" or "windows".
}

// describeGCEMachines retrieves the name, external IP and OS of all GCE machines.
func describeGCEMachines(ctx context.Context) ([]machine, error) {
	// This command produces a table with the following format:
	//
	//     name,nat_ip,licenses
	//     skia-e-gce-100,1.2.3.4,https://www.googleapis.com/compute/v1/projects/debian-cloud/global/licenses/debian-10-buster
	//     skia-e-gce-101,5.6.7.8,https://www.googleapis.com/compute/v1/projects/windows-cloud/global/licenses/windows-server-2019-dc
	//     ...
	cmd := executil.CommandContext(
		ctx,
		"gcloud",
		"compute",
		"instances",
		"list",
		"--project=skia-swarming-bots",
		"--format=csv(name, networkInterfaces[0].accessConfigs[0].natIP, disks[0].licenses[0])",
		"--filter=name~skia-e-*",
		"--sort-by=name",
	)
	outputBytes, err := cmd.CombinedOutput()
	if err != nil {
		return nil, skerr.Fmt("%s\n%s", string(outputBytes), err)
	}

	records, err := csv.NewReader(strings.NewReader(string(outputBytes))).ReadAll()
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	var machines []machine
	for _, record := range records[1:] { // Skip header record.
		// We use the machine disk's license as a proxy for the machine OS. lovisolo@ has not found
		// any other fields that carry OS information and that are present on all machines.
		license := record[2]
		os := ""
		if strings.Contains(license, "debian") {
			os = "linux"
		} else if strings.Contains(license, "windows") {
			os = "windows"
		} else {
			return nil, fmt.Errorf("The gcloud command returned an unrecognized license: %s", license)
		}

		machines = append(machines, machine{
			name:       record[0],
			externalIP: record[1],
			os:         os,
		})
	}

	return machines, nil
}

// generateHostDefinitionsBlock produces the automatically generated block to insert into the
// ssh.cfg file.
func generateHostDefinitionsBlock(machines []machine) string {
	hostDefinitionTemplate := "Host %s\n  Hostname %s"
	pieces := []string{autogeneratedBlockBegin}
	for _, machine := range machines {
		pieces = append(pieces, fmt.Sprintf(hostDefinitionTemplate, machine.name, machine.externalIP))
	}
	pieces = append(pieces, autogeneratedBlockEnd)

	return strings.Join(pieces, "\n")
}

func ifErrThenDie(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}
