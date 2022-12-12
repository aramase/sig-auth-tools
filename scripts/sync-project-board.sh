#!/usr/bin/env bash

# Copyright 2022 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Vendored from kubernetes-sigs/sig-windows-tools/hack/sync-project-board.sh
#  * commit: 09de175041779f35595dee339c2463b2eb194555
#  * link: https://github.com/kubernetes-sigs/sig-windows-tools/blob/09de175041779f35595dee339c2463b2eb194555/hack/sync-project-board.sh


set -e
set -u
set -o pipefail

## DESCRIPTION:
##
## This script queries all repos in a given github org and adds the issues
## with label 'sig/auth' to a specified project board.
##
## REREQS:
##
## This script assumes there is a github PAT in the GITHUB_TOKEN env var
## that was created with the following permissions:
##   - repo (all)
##   - read:org
##   - user (all)
##   - read:enterprise
##   - project (all)

GH_ORG=${GH_ORG:-'kubernetes'}
PROJECT_NUMBER=${PROJECT_NUMBER:-'114'}

echo "GH_ORG=${GH_ORG}"

function get_project_id_from_number() {
    project_id="$(gh api graphql -f query='
    query($org: String!, $number: Int!) {
        organization(login: $org) {
            projectV2(number: $number) {
                id
            }
        }
    }' -f org="${GH_ORG}" -F number="$1" --jq '.data.organization.projectV2.id')"
    echo "$project_id"
}

# Get project ID
project_id=$( get_project_id_from_number "$PROJECT_NUMBER" )
echo "project id for issues (number $PROJECT_NUMBER): ${project_id}"
# TODO(aramase): add PR project ID

# Get list of repos in the org
repos_json="$(gh api graphql --paginate -f query='
    query($org: String!, $endCursor: String) {
        viewer {
            organization(login: $org) {
                repositories(first:100, after: $endCursor) {
                    nodes {
                        name
                    }
                    pageInfo {
                        hasNextPage
                        endCursor
                    }
                }
            }
        }
    }' -f org="${GH_ORG}")"

repos="$(jq ".data.viewer.organization.repositories.nodes[].name" <<< "$repos_json" |  tr -d '"' )"

for repo in kubernetes
do
    echo "Looking for issues in ${GH_ORG}/${repo}"

    # TODO: paginate this query
    issues_json="$(gh api graphql -f query='
        query($org: String!, $repo: String!) {
            repository(owner: $org, name: $repo) {
                issues(last: 100, labels: ["sig/auth"], states: OPEN) {
                    totalCount
                    nodes {
                        id
                        number
                        title
                    }
                }
            }
        }' -f org="${GH_ORG}" -f repo="${repo}")"

    num_issues=$(jq ".data.repository.issues.nodes | length" <<< "$issues_json")
    echo "  found ${num_issues} in repo"

    if [ "$num_issues" -gt 0 ]; then
        range=$((num_issues - 1))
        for i in $(seq 0 $range)
        do
            issue_id=$(jq ".data.repository.issues.nodes[$i].id" <<< "$issues_json")
            issue_title=$(jq ".data.repository.issues.nodes[$i].title" <<< "$issues_json")
            issue_number=$(jq ".data.repository.issues.nodes[$i].number" <<< "$issues_json")
            echo "    adding ${issue_number} - ${issue_title}"

            # gh api graphql -f query='
            #     mutation($project:ID!, $issue:ID!) {
            #         addProjectV2ItemById(input: {projectId: $project, contentId: $issue}) {
            #             item {
            #                 id
            #             }
            #         }
            #     }' -f project="${project_id}" -f issue="${issue_id}" --jq .data.addProjectV2ItemById.item.id > /dev/null
        done
    fi
done
