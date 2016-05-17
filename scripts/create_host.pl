#!/usr/bin/perl

#apt-get install -y libnet-dns-perl  libyaml-syck-perl
use strict; 
use Data::Dumper; 
use PMCMesos; 
use YAML::Syck;
use Getopt::Std ;
   

my %qdata_opts; 

getopts("Hh:c:v:q:s:e:b:", \%qdata_opts) ; 

die print_help() if(defined $qdata_opts{H}) ; 


my $hostname = $qdata_opts{h}; 
my $ct = $qdata_opts{c};
my $vlan = $qdata_opts{v};
my $q = $qdata_opts{q};
my $size = $qdata_opts{s}; 
my $exr = $qdata_opts{e}; 
my $bm = $qdata_opts{b}; 


die print_help()  if !(defined $vlan && defined $hostname && defined $ct && defined $q) ; 
my $check_host_exists = PMCMesos::CheckDNS($hostname) ; 
die if($check_host_exists) ; 
if(defined $bm )  {
	my @doms = split/\./,$bm ; 
	if ( !(@doms > 3 && $doms[-1] =~ /^phonepe$/i) )  {
		die "Baremetal should be name.phonepe.<dom>" 
	}
}
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
$qdata{baremetal} = $bm ;


print Dumper \%qdata; 

my $client = Beanstalk::Client->new( { server => $q , default_tube => 'dnsmasq', }) or die "$!\n";
PMCMesos::UpdateQ($client,\%qdata) ; 


sub print_help {

	print "\n\t"; 
	print __FILE__. " -H # This help \n"; 
	print "\n\t"; 
	print __FILE__. " -h <hostname> -c <component type> -v <vlan> -q <beanstalk end point> [-s C1M1024] [-e <executor> ] [ -b baremetal IP ] \n\n"; 

}
