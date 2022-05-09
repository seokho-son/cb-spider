#!/bin/bash

if [ "$1" = "" ]; then
        echo
        echo -e 'usage: '$0' mock|aws|azure|gcp|alibaba|tencent|ibm|openstack|cloudit|ncp|nhncloud'
        echo -e '\n\tex) '$0' aws'
        echo
        exit 0;
fi

echo -e "#############################################"
echo -e "# TEST: $0 "
echo -e "#############################################"

source ../common/setup.env $1
source setup.env $1

echo "============== before RemoveRules: '${SG_NAME}' --- outbound:TCP/22/22, outbound:TCP/1000/1000, outbound:UDP/1/65535, outbound:ICMP/-1/-1"
#### @todo Change this command with spctl
curl -sX DELETE http://localhost:1024/spider/securitygroup/${SG_NAME}/rules -H 'Content-Type: application/json' -d \
        '{
                "ConnectionName": "'${CONN_CONFIG}'",
                "ReqInfo": {
                "RuleInfoList" :
                        [
                                {
                                        "Direction": "outbound",
                                        "IPProtocol": "TCP",
                                        "FromPort": "22",
                                        "ToPort": "22"
                                },
                                {
                                        "Direction": "outbound",
                                        "IPProtocol": "TCP",
                                        "FromPort": "1000",
                                        "ToPort": "1000"
                                },
                                {
                                        "Direction": "outbound",
                                        "IPProtocol": "UDP",
                                        "FromPort": "1",
                                        "ToPort": "65535"
                                },
                                {
                                        "Direction": "outbound",
                                        "IPProtocol": "ICMP",
                                        "FromPort": "-1",
                                        "ToPort": "-1"
                                }
                        ]
                }
        }' |json_pp

echo "============== after RemoveRules: '${SG_NAME}' --- outbound:TCP/22/22, outbound:TCP/1000/1000, outbound:UDP/1/65535, outbound:ICMP/-1/-1"

if [ "$SLEEP" ]; then
        sleep $SLEEP
else
        sleep 7
fi

# print the table header of test results
$(test_result_header $1)


#                   CSP I:TCP-01 I:TCP-02 I:UDP-01 I:ICMP-01 | O:TCP-01 O:TCP-02 O:UDP-01 O:ICMP-01
#./io-traffic-test.sh $1    $2      $3        $4       $5           $6       $7       $8      $9
./io-traffic-test.sh $1    pass    fail      skip   fail         fail     fail     skip    fail

# print the end mesg of test results
$(test_result_tailer)


echo -e "\n\n"

