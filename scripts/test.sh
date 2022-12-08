#!/bin/bash -e

ARGUMENT_LIST=(
  "num"
  "bw"
  "arg-three"
)

podNum=3

declare -A coefficients
coefficients["512"]=10
coefficients["1k"]=8
coefficients["4k"]=5
coefficients["8k"]=3
coefficients["16k"]=2
coefficients["32k"]=1

#default coefficient is 1
coefficient=1
BS=("512" "1k" "4k" "8k" "16k" "32k")


getParams() {
  # read arguments
  opts=$(getopt \
    --longoptions "$(printf "%s:," "${ARGUMENT_LIST[@]}")" \
    --name "$(basename "$0")" \
    --options "" \
    -- "$@"
  )
  eval set --$opts
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --num)
        podNum=$2
        shift 2
        ;;

      --bw)
        bw=$2
        shift 2
        ;;

      --arg-three)
        argThree=$2
        shift 2
        ;;

      *)
        break
        ;;
    esac
  done
}

getParams "$@"
echo $bw


function rand(){
    min=$1
    max=$(($2 - $min + 1))
    num=$(($RANDOM+1000000000))
    echo $(($num%$max + $min))
}

function rwmod() {
	randrw=$(rand 0 2)
    if [ $randrw -eq 0 ]
    then
    	rw=randread
    elif [ $randrw -eq 1 ]
    then
    	rw=randwrite
    else
    	rw=randrw
    fi
    echo $rw
}

# random generate randread/randwrite bw
function getRates() {
	rates=(0 0)
	if [ $1 == "randread" ]; then
		rates[0]=$(rand minBw maxBw)
	elif [ $1 == "randwrite" ]; then
		rates[1]=$(rand minBw maxBw)
	else
		rates[0]=$(rand minBw maxBw)
		rates[1]=$(rand minBw maxBw)
	fi
	echo ${rates[*]}
}

function randBs() {
	i=$(rand 1 33)
	if [ $i == 33 ]; then
		echo 512
	else
		echo $i"k"
	fi
}

function getCoefficient() {
	num=${1%?}
	for((i=1;i<${#BS[@]};i++)) do
		if [ $num -lt ${BS[i]%?} ]; then
			echo ${coefficients[${BS[`expr $i - 1`]}]}
			break
		fi
	done
}

generateCore() {
  newYaml=$1.yml
  cp template.yml $newYaml
  sed -i -e "s;%podName%;$1;g" $newYaml
  sed -i -e "s;%containerName%;$2;g" $newYaml
  sed -i -e "s;%filename%;$3;g" $newYaml
  sed -i -e "s;%iodepth%;$4;g" $newYaml
  sed -i -e "s;%rw%;$5;g" $newYaml
  sed -i -e "s;%ioengine%;$6;g" $newYaml
  sed -i -e "s;%rbs%;$7;g" $newYaml
  sed -i -e "s;%wbs%;$8;g" $newYaml
  sed -i -e "s;%size%;$9;g" $newYaml
  sed -i -e "s;%rRate%;${10};g" $newYaml
  sed -i -e "s;%wRate%;${11};g" $newYaml
  sed -i -e "s;%numjobs%;${12};g" $newYaml
  sed -i -e "s;%runtime%;${13};g" $newYaml
  sed -i -e "s;%name%;${14};g" $newYaml
}

generatePodSpecs() {
  total=$bw
  minBw=$(( $bw / $podNum / 2 ))
  maxBw=$(( $bw / $podNum))

  for ((i=1; i<$podNum;i++))
  do
    echo "generate "$i" pod"
    local podName containerName filename iodepth rw ioengine bs size rate numjobs runtime name
    podName=fio-$i
    newYaml=$podName.yml
    cp template.yml $newYaml
    containerName=con-$i
    filename=/tmp/test
    iodepth=1

    rw=`rwmod`
    ioengine=libaio
    rbs=($(randBs))
	wbs=($(randBs))

    size=10g
    rates=($(getRates $rw))
    coefficient=$(getCoefficient "31k")
    eval set $rates
    echo $rates
    echo $total ${rates[0]} $coefficient
    total=$(($total - ${rates[0]} * coefficient))
	echo $total
    numjobs=1
    runtime=604800
    name=test-$i
    generateCore $podName $containerName $filename $iodepth $rw $ioengine $rbs $wbs $size ${rates[0]} ${rates[1]} $numjobs $runtime $name
  done

  echo "generate $podNum pod"
  podName=fio-$podNum
  newYaml=$podName.yml
  cp template.yml $newYaml
  containerName=con-$podNum
  name=test-$podNum
  rw=`rwmod`
  bs=($(randBs))
  echo $total

  generateCore $podName $containerName $filename $iodepth $rw $ioengine $bs $bs $size $total $total $numjobs $runtime $name
}

generatePodSpecs
