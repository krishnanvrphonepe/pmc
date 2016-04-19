#!/usr/bin/perl

#apt-get install -y libnet-dns-perl  libyaml-syck-perl
use strict; 
use Data::Dumper; 
use PMC; 
use YAML::Syck;
   


my $hostname = shift; 
my $ct = shift; 
my $vlan = shift  ; 
my $size = shift; 
die __FILE__." <hostname> <ct> <vlan> [size]\n" if !(defined $vlan && defined $hostname && defined $ct ) ; 
my $sizef = PMC::VerifyValidSize($size) ; 
die "Invalid size\n" if(!$sizef) ;
my $host_ip = PMC::GetFreeIP($vlan);
my $mac = PMC::GenMAC($host_ip) ;
$size = 'C1M1024' if(!defined $size) ; 
my %qdata; 

$qdata{hostname} = $hostname ; 
$qdata{mac} = $mac; 
$qdata{ip} = $host_ip ; 
$qdata{cpu} = PMC::GetCPU($size)  ;
$qdata{mem} = PMC::GetMemory($size) ;
$qdata{ct} = $ct ;

print Dumper \%qdata; 

PMC::UpdateQ("localhost",\%qdata,"dnsmasq") ; 
