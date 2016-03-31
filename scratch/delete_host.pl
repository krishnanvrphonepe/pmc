#!/usr/bin/perl

use strict; 
use XML::Simple; 
use Data::Dumper; 
use Data::Validate::IP qw (is_private_ipv4) ; 
use Net::DNS ; 
use Net::Ping ; 
use YAML::Syck; 
   

my $PMCDIR = '/var/local/pmc'; 
my $HOST_IMAGE_LOCATION = '/var/lib/libvirt/images'; 

my $hostname = shift; 
my $ip = shift; 

die __FILE__." <hostname> <ip>\n" if !(defined $ip && defined $hostname) ; 


my $cmd = `virsh net-dumpxml default`; 
#print Dumper $cmd ; 
my $xml_ref = XMLin( $cmd, KeepRoot => 1, ForceArray => 1,);


my $ipf = VerifyValidInput($hostname,$ip,$xml_ref->{network}->[0]->{ip}->[0]->{dhcp}->[0]->{host}) ;
die if(!$ipf) ;
GenerateNetworkConfig($hostname,$ip,$PMCDIR.'/default.xml') ; 
DeleteAndDestroyVM($hostname) ; 
DeleteHostImages($hostname) ; 


sub VerifyValidInput {
	my $hnf = shift;
	my $ip = shift; 
	my $xml_h = shift ; 
	foreach my $hn (keys %$xml_h) {
		next if($hnf ne $hn) ; 
		print "NOW $hn\n"; 
		if($xml_h->{$hn}{ip} eq $ip) {
			print STDERR __LINE__.": $ip: Already defined in DHCP Config\n"; 
			return 1; 
		}
	}
	print STDERR __LINE__.": Invalid host / IP combination ($hnf,$ip) \n"; 

	return 0; 

}

sub GenerateNetworkConfig { 
	my $hn =  shift; 
	my $ip = shift ; 
	my $f = shift ; 
	delete $xml_ref->{network}->[0]->{ip}->[0]->{dhcp}->[0]->{host}->{$hostname} ;
	XMLout( $xml_ref, KeepRoot => 1, NoAttr => 0, OutputFile => "/tmp/default.xml");
	my $net_start = `virsh net-define /tmp/default.xml && virsh net-destroy default && virsh net-start default` ; 
	print "NETSTART\n$net_start\n\n"; 
} 



sub DeleteAndDestroyVM {
	my $hn = shift ; 
	my $dnsf = `virsh destroy $hn  ; virsh undefine $hn`; 
	print $dnsf; 
}

sub DeleteHostImages {
	my $hn = shift; 
	unlink $HOST_IMAGE_LOCATION."/${hn}.img" ; 
	unlink "${PMCDIR}/hostdata.${hn}" ;
	unlink "${PMCDIR}/hostdata.${hn}.img" ;
	unlink "${PMCDIR}/${hn}.xml"; 
}
