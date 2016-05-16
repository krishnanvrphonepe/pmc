#!/usr/bin/perl

#apt-get install -y libnet-dns-perl  libyaml-syck-perl
use strict; 
use Data::Dumper; 
use PMCMesos; 
use YAML::Syck;
   


my $hostname = shift; 
my $ct = shift; 
my $vlan = shift  ; 
my $q = shift  ; 
my $size = shift; 
my $exr = shift; 
die __FILE__." <hostname> <ct> <vlan> <q> [size] [executor]\n" if !(defined $vlan && defined $hostname && defined $ct && defined $q) ; 
my $sizef = PMCMesos::VerifyValidSize($size) ; 
die "Invalid size : $size\n" if(!$sizef) ;
my $host_ip = PMCMesos::GetFreeIP($vlan);
my $mac = PMCMesos::GenMAC($host_ip) ;
$size = 'C1M1024' if(!defined $size) ; 
my %qdata; 

$qdata{hostname} = $hostname ; 
$qdata{mac} = $mac; 
$qdata{ip} = $host_ip ; 
$qdata{cpu} = PMCMesos::GetCPU($size)  ;
$qdata{mem} = PMCMesos::GetMemory($size) ;
$qdata{comp_type} = $ct ;
$qdata{executor} = $exr ;

print Dumper \%qdata; 

my $client = Beanstalk::Client->new( { server => $q , default_tube => 'dnsmasq', }) or die "$!\n";
PMCMesos::UpdateQ($client,\%qdata) ; 
