#!/usr/bin/perl

use Beanstalk::Client;
use PMC;
use Data::Dumper; 
my $server = '192.168.254.1' ; 
my $tube = 'dnsmasq' ; 
$| = 1; 

my $client = Beanstalk::Client->new( { server => $server , default_tube => 'dnsmasq', }) or die "$!\n";
my $mesos_client = Beanstalk::Client->new( { server => $server , default_tube => 'mesos', }) or die "$!\n";

for(;;) {
	print "Sleeping\n"; 
	sleep 2; 

	my $qdatar = PMC::FetchMsgFromQ($client) ; 
	my $qdata = $qdatar->{DATA}; 
	#host=a ip=192.168.254.15 mac=52:54:00:a8:fe:0f cpu=2 mem=2097152 ct=b
	print Dumper $qdata; 
	PMC::GenerateNetworkConfig($qdata->{hostname},$qdata->{ip},$qdata->{mac}) ; 
	$qdatar->{JOB}->delete($qdatar->{JOB}->id());
	print Dumper \%qdata;
	PMC::UpdateQ($mesos_client,$qdata) ; 
}
