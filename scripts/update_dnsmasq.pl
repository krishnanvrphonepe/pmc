#!/usr/bin/perl

use Beanstalk::Client;
use PMC;
use Data::Dumper; 
my $server = 'localhost' ; 
my $tube = 'dnsmasq' ; 
$| = 1; 

my $client = Beanstalk::Client->new( { server => '192.168.254.1', default_tube => 'dnsmasq', }) or die "$!\n";

for(;;) {
	print "Sleeping\n"; 
	sleep 2; 

	my $jobc = $client->reserve;
	print "Got here\n"; 
	next unless $jobc ;
	my @args = $jobc->args() ;
	print "@args" ;
	my %qdata; 
	foreach(@args) {
		my($k,$v) = split/=/; 
		$qdata{$k} = $v ; 
	}
	print Dumper \%qdata;
	#host=a ip=192.168.254.15 mac=52:54:00:a8:fe:0f cpu=2 mem=2097152 ct=b
	PMC::GenerateNetworkConfig($qdata{host},$qdata{ip},$qdata{mac}) ; 
	print "GOT here\n"; 
	$jobc->delete();
	PMC::UpdateQ("localhost",\%qdata,"mesos") ; 
	print "Sleeping\n"; 
}
