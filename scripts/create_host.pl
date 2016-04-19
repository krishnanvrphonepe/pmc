#!/usr/bin/perl

#apt-get install -y libnet-dns-perl  libyaml-syck-perl
use strict; 
use Data::Dumper; 
use PMC; 
use YAML::Syck;
   


my $hostname = shift; 
my $ct = shift; 
my $vlan = shift  ; 
my $q = shift  ; 
my $size = shift; 
my $exr = shift; 
die __FILE__." <hostname> <ct> <vlan> <q> [size] [executor]\n" if !(defined $vlan && defined $hostname && defined $ct && defined $q) ; 
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
$qdata{comp_type} = $ct ;
$qdata{executor} = $exr ;

print Dumper \%qdata; 

PMC::UpdateQ($q,\%qdata,"dnsmasq") ; 
